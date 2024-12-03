.PHONY: build_all build_test_template build_order build_payment

up: build_all 
	docker-compose -f config/docker-compose.yml up

down:  
	docker-compose -f config/docker-compose.yml down -v

build_all: build_test_template build_order build_payment

build_test_template: # TODO: turn into sanity check. 
	docker build -t workload:v1 -f test/opt/antithesis/test/v1/basic/Dockerfile ./test/opt/antithesis/test/v1/basic

build_order:
	docker build -t order:v1 -f orderService/Dockerfile ./orderService

build_payment:
	docker build -t payment:v1 -f paymentService/Dockerfile ./paymentService
