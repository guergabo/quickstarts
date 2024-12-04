# Test Composer 

The test composer takes care of varying parallelism, test length, and command order. 

It works by running executables that you provide, called test commands. 

The filename of each test commands starts with a prefix, which tells the Test Composer how the command should run. 

(drivers > (workload+API))

## Hierarchy 

- Directory represents a collection of test commands that are compatible with one another, meaning they might run alongside each other and not be isolated. 

- Test command is an executable for the test composer to run. It may have any or no extension, but must be marked executable by the container's default user (way to tests). Prefix specifies the type of test command. 

## Test Command Types 

### Driver Comands 

After `first` command and during a period where Antithesis may be adding faults to the test environment.

1. `singleton_driver_`: May run as the only driver command in a particular branch of history. Non-driver commands like `first` and `finally` will still run. 

2. `serial_driver_`: May run at any poitn after the `first` command (if any) when there are no other driver commands runnings. Before or after `parallel_driver` or `serial_driver` commands, but they will not be run alongside either. Non-driver commands like `anytime` will still run alongside. Good example: self-validating step, also any instruction that would be hindered by other things running in parallel.

3. `parallel_driver_`: May run while other `prallel_driver` commands are running. A "data source" that publishes to a queue. Think though the implications of that concurrency. (SEEMS WRITE FOCUSED). To start things? but that's seems nice fort first_

### Quiescent Commands 

1. `first_`: May run to leave the system in an appropriate state for the driver commands. Nice to setting up data or bootstrapping state for late command. 
2. `eventually_`: May run after at least one driver command has started. Create a new branch in which all driver comands are killed and Antithesis no longer injects new faults. Last command, may be destructive. Ideal for testing eventual properties of a systems, like availability or replica consistency. Must check for where faults are occuring..

### Advanced Commands 

1. `finally_`: May only run in a branch where every driver command that was started has also completed succesfully.
2. `anytime_`: May run at any time after the `first` command, including during `singleton` or `serial` driver commands. (SPECIAL OVERLAP, other one is parallel, but with its sel not aong serial and singleton).

## Dockerfile 

```
make build_test_template 
docker run -it --entrypoint /bin/bash workload:v1
```
