.PHONY: up down build_all build_config build_services build_order build_payment build_test_template

up: build_all 
	docker-compose -f config/docker-compose.yml up

down:  
	docker-compose -f config/docker-compose.yml down -v

build_all: build_config

build_config: build_services
	docker build -t config:v1 -f config/Dockerfile ./config

build_services: build_order build_payment build_test_template

build_order:
	docker build -t order:v1 -f orderService/Dockerfile ./orderService

build_payment:
	docker build -t payment:v1 -f paymentService/Dockerfile ./paymentService

build_test_template: # TODO: turn into useful sanity check. 
	docker build -t workload:v1 -f test/opt/antithesis/test/v1/Dockerfile ./test/opt/antithesis/test/v1/
