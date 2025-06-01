apply:
	cd infra && tofu apply

tidy:
	cd lambdas/linksCRUD && go mod tidy
	cd lambdas/golinksbrowser && go mod tidy
	cd lambdas/linkguesser && go mod tidy
	cd shared && go mod tidy

build_golinks_browser:
	rm -f builds/go_links_browser.zip
	rm -f builds/go_links_browser
	cd lambdas/golinksbrowser && GOOS=linux GOARCH=arm64 go build -o bootstrap
	cd lambdas/golinksbrowser && zip ../../builds/go_links_browser.zip bootstrap && rm bootstrap

build_linkscrud:
	rm -f builds/links_crud.zip
	rm -f builds/links_crud
	cd lambdas/linkscrud && GOOS=linux GOARCH=arm64 go build -o bootstrap
	cd lambdas/linkscrud && zip ../../builds/links_crud.zip bootstrap && rm bootstrap

build_linkguesser:
	rm -f builds/linkguesser.zip
	rm -f builds/linkguesser
	cd lambdas/linkguesser && GOOS=linux GOARCH=arm64 go build -o bootstrap
	cd lambdas/linkguesser && zip ../../builds/linkguesser.zip bootstrap && rm bootstrap

build: build_linkscrud build_golinks_browser build_linkguesser
