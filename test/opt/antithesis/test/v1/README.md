# Test Composer

The Test Composer is part of the Antithesis platform. It helps maximize bug detection by varying:
- Test parallelism
- Test length 
- Command execution order

## How It Works

The Test Composer executes test commands that you provide. Each test command's filename must start with a specific prefix that indicates how the command should be executed. For all options, please see our [documentation](https://www.antithesis.com/docs/test_templates/test_composer_reference/).

## Directory Structure

### basic/

**Goal**: Verify correct producer writing behavior
- Uses `Singleton` command

### intermediate/

**Goal**: Verify consumer message processing and internal assertions
- Uses `parallel` and `finally` commands
