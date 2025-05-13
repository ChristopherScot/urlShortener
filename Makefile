
apply:
	cd infra && tofu apply

tidy:
	cd lambdas/createlink && go mod tidy
	cd lambdas/getlinks && go mod tidy
	cd lambdas/golinksbrowser && go mod tidy

build_golinks_browser:
	rm -f builds/go_links_browser.zip
	rm -f builds/go_links_browser
	cd lambdas/golinksbrowser && GOOS=linux GOARCH=arm64 go build -o bootstrap
	cd lambdas/golinksbrowser && zip ../../builds/go_links_browser.zip bootstrap && rm bootstrap

build_create_link:
	rm -f builds/create_link.zip
	rm -f builds/create_link
	cd lambdas/createlink && GOOS=linux GOARCH=arm64 go build -o bootstrap
	cd lambdas/createlink && zip ../../builds/create_link.zip bootstrap && rm bootstrap

build_get_links:
	rm -f builds/get_links.zip
	rm -f builds/get_links
	cd lambdas/getlinks && GOOS=linux GOARCH=arm64 go build -o bootstrap
	cd lambdas/getlinks && zip ../../builds/get_links.zip bootstrap && rm bootstrap

build: build_create_link build_get_links build_golinks_browser