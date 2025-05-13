# JUnit Example Project

This is an example Java project that demonstrates various JUnit test scenarios, including nested tests and tests with delays. It's designed to generate JUnit XML reports for testing OpenTelemetry conversion.

## Project Structure

- `Calculator.java`: A simple calculator with basic operations and a time-delayed complex calculation method
- `StringUtils.java`: String utility methods including a time-delayed processing method
- Test classes with nested structures and various test scenarios:
  - `CalculatorTest.java`: Tests for Calculator class with nested test classes
  - `StringUtilsTest.java`: Tests for StringUtils with parameterized tests and method sources

## Running the Tests

To run the tests and generate a JUnit XML report:

```bash
cd example
mvn test
```

The JUnit XML reports will be generated in the `target/surefire-reports` directory.

## Report Features

The tests in this project are specifically designed to:

1. Demonstrate nested test structures (using `@Nested` JUnit annotations)
2. Include tests that take time to run (1-2 seconds per test)
3. Show a variety of test types (simple, parameterized, exception testing)
4. Generate comprehensive JUnit XML reports with various duration values

## Example Test Output

```
Running com.example.CalculatorTest
Tests run: 10, Failures: 0, Errors: 0, Skipped: 0, Time elapsed: 7.321 s
Running com.example.StringUtilsTest
Tests run: 15, Failures: 0, Errors: 0, Skipped: 0, Time elapsed: 8.456 s

Results:
Tests run: 25, Failures: 0, Errors: 0, Skipped: 0
```

Note that the exact output and timing will vary based on your system's performance.