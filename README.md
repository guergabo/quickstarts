# quickstarts

Antithesis quickstart code sample and tutorials showcasing core Antithesis capabilities

## Application 

- Golang microservices with Chi
- Postgres 
- NATs
- Stripe API

Start: 

```console
make up
```
Stop: 

```consle
make down 
```

#### Order Service 

Create purchse order: 

```console 
curl -X POST \
  http://localhost:8000/orders \
  -H 'Content-Type: application/json' \
  -d '{
    "amount": 99.99
		"currency": "usd",
    "customer": "Alice_123",
		"description": "This is demo"
  }'

  curl -X GET http://localhost:8000/orders/1
```

Get purchase order: 

```console 
curl -X GET http://localhost:8000/orders/1
```

#### Payment Service 

Consuming message using NATs (Jetstream) with their pull model and writes to Stripe API (stripe-mock).

#### Workload 

Basic.
