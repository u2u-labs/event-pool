.PHONY: api1 clean proto

api1:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool serve

clean:
	rm -f event-pool
	rm -rvf ./data/*/blockchain ./data/*/trie

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/event.proto

run:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool server --config config.yaml --grpc-address :10000 --libp2p :10006 --jsonrpc :10002

init:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool secrets init --data-dir ./data/chain

inittest:
	rm -rf ./data/test
	go build -ldflags -w
	chmod +x event-pool
	./event-pool secrets init --data-dir ./data/test

runtest: inittest
	./event-pool server --data-dir ./data/test --grpc-address :10100 --libp2p :10101 --jsonrpc :10102 --config config.yaml
