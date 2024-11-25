.PHONY: build_all build_workload build_order build_payment

up: build_all 
	docker-compose -f config/docker-compose.yml up

down:  
	docker-compose -f config/docker-compose.yml down -v

build_all: build_workload build_order build_payment

build_workload:
	docker build -t workload:v1 -f workload/Dockerfile ./workload

build_order:
	docker build -t order:v1 -f orderService/Dockerfile ./orderService

build_payment:
	docker build -t payment:v1 -f paymentService/Dockerfile ./paymentService
