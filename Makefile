.PHONY: fib
fib:
	go run ./cmd/fib

.PHONY: zipkin
zipkin:
	docker run -d -p 9411:9411 openzipkin/zipkin
