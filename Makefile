NAME := moex-currency-bot
all: build

build:
	go build -C ./src/ -o ../build/$(NAME)

test:
	go test -C ./src/

clean:
	rm -rf ./build/

docker:
	docker build . -t $(NAME)

docker-clean:
	docker rmi $(NAME)

fix-perm:
	chmod 600 ./config/*.env
	chmod 600 ./certs/*.key
