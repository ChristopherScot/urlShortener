build:
	rm -f builds/create_link.zip
	rm -f builds/create_link
	cd lambdas/createlink && GOOS=linux GOARCH=arm64 go build -o bootstrap
	cd lambdas/createlink && zip ../../builds/create_link.zip bootstrap && rm bootstrap
	rm -f builds/get_links.zip
	rm -f builds/get_links
	cd lambdas/getlinks && GOOS=linux GOARCH=arm64 go build -o bootstrap
	cd lambdas/getlinks && zip ../../builds/get_links.zip bootstrap && rm bootstrap

apply:
	cd infra && tofu apply

tidy:
	cd lambdas/createlink && go mod tidy
	cd lambdas/getlinks && go mod tidy
