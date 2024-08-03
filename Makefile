run: build
	@clear && ./bin/chat

build:
	@clear && go build -o ./bin/chat .
