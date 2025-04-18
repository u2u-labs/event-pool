.PHONY: api1 clean proto

api1:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool serve

clean:
	rm -f event-pool

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/event.proto

run:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool server --config config.yaml

init:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool secrets init --data-dir ./data/temp
