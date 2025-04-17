.PHONY: api1 clean

api1:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool serve

clean:
	rm -f event-pool

run:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool server --config config.yaml

init:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool secrets init --data-dir ./data/temp
