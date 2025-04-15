.PHONY: api1 clean

api1:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool serve

clean:
	rm -f event-pool

node:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool run
