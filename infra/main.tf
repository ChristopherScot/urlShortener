variable "token" {
  description = "The token used for authenticating Lambda requests"
  type        = string
}

provider "aws" {
  region = "us-east-1"
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
}

resource "aws_s3_bucket_policy" "public_read" {
  bucket = aws_s3_bucket.redirects.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.redirects.arn}/go/*"
      }
    ]
  })
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

resource "aws_lambda_function" "create_redirect" {
  filename         = "../builds/create_link.zip"
  function_name    = "create-link"
  role             = aws_iam_role.lambda_exec.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]

  source_code_hash = filebase64sha256("../builds/create_link.zip")

  environment {
    variables = {
      BUCKET_NAME    = aws_s3_bucket.redirects.bucket
      DYNAMODB_TABLE = "Redirects"
      TOKEN          = var.token
    }
  }
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
        ]
        Resource = aws_dynamodb_table.redirects.arn
      }
    ]
  })
}

# Define the get-links Lambda function
resource "aws_lambda_function" "get_links" {
  filename         = "../builds/get_links.zip"
  function_name    = "get-links"
  role             = aws_iam_role.lambda_exec.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]

  source_code_hash = filebase64sha256("../builds/get_links.zip")

  environment {
    variables = {
      DYNAMODB_TABLE = aws_dynamodb_table.redirects.name
      TOKEN               = var.token
    }
  }
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