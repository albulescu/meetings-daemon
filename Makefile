run:
	go run main.go -config=config.ini

install:
	go get gopkg.in/mgo.v2;
	go get gopkg.in/mgo.v2/bson;
	go get github.com/sevlyar/go-daemon;
	go get github.com/alexjlockwood/gcm;
	go get gopkg.in/ini.v1

build:
	go build -o $(GOPATH)/bin/osx-omeetings main.go;
	GOOS=linux GOARCH=386 CGO_ENABLED=0 go build -o $(GOPATH)/bin/omeetings main.go;
