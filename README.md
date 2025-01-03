

#### **Getting Started**

# **Quickstart**

Welcome to Antithesis\! Let’s get started with Antithesis in 6 quick steps.

## **Prerequisites**

* Docker installed and running.   
* Credentials to access the Antithesis registry.  
* Credentials to access the Antithesis platform.  
* Homebrew.  
* Golang.  
* Basic familiarity with microservices. 

## 1\) Install the Antithesis CLI

We’ll be using the Antithesis CLI throughout this quickstart to create test runs.

```console 
brew tap guergabo/antithesis && brew install antithesis
```

Verify your installation and see some cool ASCII art: 

```console 
antithesis
```

## 2\) Initialize an Antithesis project

Create a dedicated directory:

```console 
mkdir antithesis && cd antithesis
````

Now the next command will download and set up a preconfigured Antithesis project:

```console 
antithesis init quickstart .
```

Change to the new directory to see the project:

```console 
cd quickstart
```

The demo project includes two Go microservices (Order and Payment) that interact with Postgres, NATS, and a Stripe mock \- perfect for learning Antithesis’ core features.

## 3\) Build and push your first test environment

First, set up authentication to access the Antithesis private image registry. This allows you to pull and push container images required for your test environment. 

Create the Antithesis configuration directory: 

```console
mkdir -p ~/.config/antithesis
``` 

Move your service account credentials to the config directory:
```console
mv service-account.json ~/.config/antithesis
``` 

Configure authentication by setting the credentials as an environment variable:
```console
export ANTITHESIS_GAR_KEY=$(cat ~/.config/antithesis/service-account.json)
``` 

Once authentication is configured, build and push your test environment to Antithesis:

```console
make env
```

## 4\) Create your first Antithesis Test Run

Now--time to create your first test run using the following command (replace `tenant`, `username`, `password`, and `email` with your values):

```console
antithesis run \
  --name='quickstart' \
  --description='Running a quick antithesis test.' \
  --tenant='YOUR_TENANT' \
  --username='YOUR_USERNAME' \
  --password='YOUR_PASSWORD' \
  --config='us-central1-docker.pkg.dev/molten-verve-216720/ant-pdogfood-repository/config:latest' \
  --image='us-central1-docker.pkg.dev/molten-verve-216720/ant-pdogfood-repository/order:latest' \
  --image='us-central1-docker.pkg.dev/molten-verve-216720/ant-pdogfood-repository/payment:latest' \
  --image='us-central1-docker.pkg.dev/molten-verve-216720/ant-pdogfood-repository/test-template:latest' \
  --image='docker.io/postgres:16' \
  --image='docker.io/nats:latest' \
  --image='docker.io/stripemock/stripe-mock:latest' \
  --duration=15 \
  --email='YOUR_EMAIL'
```

## 5\) Understanding what’s happening

Congratulations on launching your first test run\! While the test is running (it takes approximately 30 minutes to generate the complete report), take this time to explore the project and review our documentation. This will help you build a mental model of how everything works. Here are a few suggestions:

* [How Antithesis Works](https://www.antithesis.com/docs/introduction/how_antithesis_works/)  
* [Antithesis SDKs](https://www.antithesis.com/docs/using_antithesis/sdk/) (*Pro tip: Search the project’s codebase for assert.\*, random.\*, and lifecycle.\* to see our SDKs in action*)  
* [Instrumenting your code](https://www.antithesis.com/docs/instrumentation/) (*Pro tip: peek into the Dockerfiles).*  
* [Antithesis Test Composer](https://www.antithesis.com/docs/test_templates/) (*Pro tip: check out test/opt/antithesis/v1/\**)   
* Notice how the config image is simply packaging up your docker-compose.
* Notice how we have to mock third-party dependencies like Stripe. (Pro tip: peek at the docker-compose.yml)
* Notice how test duration is configurable. You’re no longer specifying test cases but thinking in terms of test hours.
* [GitHub Action Integration](https://www.antithesis.com/docs/using_antithesis/ci/) (*Pro tip: peek into the ci.yml*)  


## 6\) View Antithesis Test Report

After 30 minutes, you should receive a test report in your email that looks like the image below. To interpret the results, please refer to our [documentation on test reports](https://www.antithesis.com/docs/reports/triage/). (*Pro tip: search the project's codebase for "BUG:" and you will find the 3 Always assertion failing*)

<img width="1506" alt="Screenshot 2025-01-03 at 2 32 30 AM" src="https://github.com/user-attachments/assets/d8090ec3-d138-4ca4-a710-7401bf2221f3" />

One notable feature is the ability to view not only the current run's results but also the recent changes, including when a property last failed. As you can see from my track record, I've been practicing classic Test-Driven Development - with plenty of stumbles along the way. Let's just say surviving in Antithesis' environment requires both persistence and a high tolerance for failure messages. The road to passing tests is paved in red.

## What's next?

Feel free to play around with the quickstart. Your development workflow will look like this: 

1. Make changes to the code   
2. Repeat steps 3 and 4\.

Feel ready to build your own application? Choose from our available SDKs:

* [Go SDK](https://www.antithesis.com/docs/using_antithesis/sdk/go/)   
* [Rust SDK](https://www.antithesis.com/docs/using_antithesis/sdk/rust/)   
* [C++ SDK](https://www.antithesis.com/docs/using_antithesis/sdk/cpp/)   
* [Python SDK](https://www.antithesis.com/docs/using_antithesis/sdk/python/)  
* [Java SDK](https://www.antithesis.com/docs/using_antithesis/sdk/java/)
