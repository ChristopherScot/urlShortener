variable "token" {
  description = "The token used for authenticating Lambda requests"
  type        = string
}

variable "OpenAIAPIKey" {
  description = "Key for OpenAPI integration"
  type        = string
}


provider "aws" {
  region = "us-east-1"

  default_tags {
    tags = {
      Environment = "Production"
      Project     = "URLShortener"
    }
  }
}

resource "aws_s3_bucket" "redirects" {
  bucket = "go-shortener-redirects"
}

resource "aws_s3_bucket_public_access_block" "redirects" {
  bucket = aws_s3_bucket.redirects.id

  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

resource "aws_s3_bucket_website_configuration" "redirects" {
  bucket = aws_s3_bucket.redirects.id     

  index_document {
    suffix = "index.html"
  }

  error_document {
    key = "go/404.html"
  }
}


resource "aws_iam_role_policy" "lambda_logging" {
  name   = "create-link-lambda-logging-policy"
  role   = aws_iam_role.lambda_exec.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:*:*:*"
      }
    ]
  })
}

resource "aws_iam_role" "lambda_exec" {
  name = "create-link-lambda-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
        Effect = "Allow"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_s3_access" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3FullAccess"
}


resource "aws_dynamodb_table" "redirects" {
  name           = "Redirects"
  billing_mode   = "PAY_PER_REQUEST" 
  hash_key       = "Alias"           

  attribute {
    name = "Alias"
    type = "S" # String
  }

  tags = {
    Environment = "Production"
    Project     = "URLShortener"
  }
}

resource "aws_iam_role_policy" "lambda_dynamodb_access" {
  name   = "create-link-lambda-dynamodb-policy"
  role   = aws_iam_role.lambda_exec.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = [
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem",
          "dynamodb:BatchGetItem",
        ]
        Resource = aws_dynamodb_table.redirects.arn
      }
    ]
  })
}


# Add IAM policy for get-links Lambda to access DynamoDB
resource "aws_iam_role_policy" "lambda_dynamodb_get_access" {
  name   = "get-links-lambda-dynamodb-policy"
  role   = aws_iam_role.lambda_exec.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = [
          "dynamodb:GetItem",
          "dynamodb:Scan"
        ]
        Resource = aws_dynamodb_table.redirects.arn
      }
    ]
  })
}
resource "aws_lambda_function" "links_crud" {
  filename         = "../builds/links_crud.zip"
  function_name    = "links-crud"
  role             = aws_iam_role.lambda_exec.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]

  source_code_hash = filebase64sha256("../builds/links_crud.zip")

  environment {
    variables = {
      BUCKET_NAME    = aws_s3_bucket.redirects.bucket
      DYNAMODB_TABLE = aws_dynamodb_table.redirects.name
      TOKEN          = var.token
    }
  }
}

resource "aws_lambda_function" "go_links_browser" {
  filename         = "../builds/go_links_browser.zip" # Path to the zipped Lambda code
  function_name    = "go-links-browser"
  role             = aws_iam_role.lambda_exec.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]

  source_code_hash = filebase64sha256("../builds/go_links_browser.zip")

  environment {
    variables = {
      LINKS_CRUD_LAMBDA = aws_lambda_function.links_crud.function_name
      TOKEN              = var.token
    }
  }
}

resource "aws_s3_bucket_policy" "redirects_bucket_policy" {
  bucket = aws_s3_bucket.redirects.id
  # For actual use restrict to your subnet's CIDR block or your NAT's IP
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
    {
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.redirects.arn}/go/*"
    },
    {
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.redirects.arn}/index.html"
    },

    ]
  })

  depends_on = [ 
    aws_s3_bucket.redirects, 
    ]
}


resource "aws_iam_role_policy" "lambda_invoke_permissions" {
  name   = "webpage-lambda-invoke-policy"
  role   = aws_iam_role.lambda_exec.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "lambda:InvokeFunction"
        Resource = [
          aws_lambda_function.links_crud.arn
        ]
      }
    ]
  })
}

resource "aws_apigatewayv2_api" "go_links_api" {
  name          = "go-links-browser-api"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_integration" "go_links_browser_integration" {
  api_id             = aws_apigatewayv2_api.go_links_api.id
  integration_type   = "AWS_PROXY"
  integration_uri    = aws_lambda_function.go_links_browser.invoke_arn
  payload_format_version = "2.0"
}

resource "aws_apigatewayv2_route" "go_links_browser_route" {
  api_id    = aws_apigatewayv2_api.go_links_api.id
  route_key = "GET /"

  target = "integrations/${aws_apigatewayv2_integration.go_links_browser_integration.id}"
}

resource "aws_apigatewayv2_route" "go_links_post_route" {
  api_id    = aws_apigatewayv2_api.go_links_api.id
  route_key = "POST /"

  target = "integrations/${aws_apigatewayv2_integration.go_links_browser_integration.id}"
}

resource "aws_apigatewayv2_route" "go_links_options_route" {
  api_id    = aws_apigatewayv2_api.go_links_api.id
  route_key = "OPTIONS /"

  target = "integrations/${aws_apigatewayv2_integration.go_links_browser_integration.id}"
}

resource "aws_apigatewayv2_stage" "go_links_stage" {
  api_id      = aws_apigatewayv2_api.go_links_api.id
  name        = "$default"
  auto_deploy = true
}

resource "aws_lambda_permission" "go_links_browser_permission" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.go_links_browser.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.go_links_api.execution_arn}/*"
}

output "go_links_browser_url" {
  description = "The base URL for the go-links-browser web UI"
  value       = "https://${aws_apigatewayv2_api.go_links_api.id}.execute-api.${data.aws_region.current.name}.amazonaws.com/"
}

 output "redirects_bucket_website_url" {
  description = "The S3 website endpoint for redirects"
  value       = "http://${aws_s3_bucket.redirects.bucket}.s3-website-${data.aws_region.current.name}.amazonaws.com/"
}

data "aws_region" "current" {}

resource "aws_lambda_function" "linkguesser" {
  filename         = "../builds/linkguesser.zip"
  function_name    = "linkguesser"
  role             = aws_iam_role.lambda_exec.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  source_code_hash = filebase64sha256("../builds/linkguesser.zip")

  environment {
    variables = {
      DYNAMODB_TABLE = aws_dynamodb_table.redirects.name
      OPENAI_API_KEY = var.OpenAIAPIKey
    }
  }
}

resource "aws_lambda_permission" "linkguesser_api" {
  statement_id  = "AllowAPIGatewayInvokeLinkguesser"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.linkguesser.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.go_links_api.execution_arn}/*"
}
resource "aws_apigatewayv2_integration" "linkguesser" {
  api_id                  = aws_apigatewayv2_api.go_links_api.id
  integration_type        = "AWS_PROXY"
  integration_uri         = aws_lambda_function.linkguesser.invoke_arn
  payload_format_version  = "2.0"
}

resource "aws_apigatewayv2_route" "linkguesser" {
  api_id    = aws_apigatewayv2_api.go_links_api.id
  route_key = "GET /linkguesser"
  target    = "integrations/${aws_apigatewayv2_integration.linkguesser.id}"
}

resource "aws_s3_object" "redirects_404" {
  bucket = aws_s3_bucket.redirects.id
  key    = "go/404.html"
  content = <<EOF
<html>
  <head>
    <meta http-equiv="refresh" content="0; url=https://${aws_apigatewayv2_api.go_links_api.id}.execute-api.${data.aws_region.current.name}.amazonaws.com/linkguesser" />
    <script>
      var path = window.location.pathname.replace(/^\/go\//, "");
      window.location.replace("https://${aws_apigatewayv2_api.go_links_api.id}.execute-api.${data.aws_region.current.name}.amazonaws.com/linkguesser?path=" + encodeURIComponent(path));
    </script>
  </head>
  <body>
    <p>Redirecting...</p>
  </body>
</html>
EOF
  content_type = "text/html"
}

resource "aws_s3_object" "redirects_index" {
  bucket = aws_s3_bucket.redirects.id
  key    = "go/index.html"
  content = <<EOF
<html>
  <head>
    <title>Go Shortener</title>
    <meta http-equiv="refresh" content="0; url=https://${aws_apigatewayv2_api.go_links_api.id}.execute-api.${data.aws_region.current.name}.amazonaws.com/" />
    <script>window.location.replace("https://${aws_apigatewayv2_api.go_links_api.id}.execute-api.${data.aws_region.current.name}.amazonaws.com/");</script>
  </head>
  <body>
    <p>Redirecting...</p>
  </body>
</html>
EOF
  content_type = "text/html"
}