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
	./event-pool run

generate:
	go build -ldflags -w
	chmod +x event-pool
	./event-pool generate
