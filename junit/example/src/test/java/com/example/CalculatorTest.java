package com.example;

import static org.junit.jupiter.api.Assertions.*;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;

@DisplayName("Calculator Tests")
class CalculatorTest {

  private Calculator calculator;

  @BeforeEach
  void setUp() {
    calculator = new Calculator();
  }

  @Test
  @DisplayName("1 + 1 = 2")
  void addSimpleTest() {
    assertEquals(2, calculator.add(1, 1), "1 + 1 should equal 2");
  }

  @Test
  @DisplayName("2 + 2 = 5")
  void addSimpleTestFailing() {
    assertEquals(5, calculator.add(2, 2), "2 + 2 should equal 5");
  }

  @Nested
  @DisplayName("Basic Operations")
  class BasicOperations {

    @Test
    @DisplayName("5 - 3 = 2")
    void subtractTest() {
      assertEquals(2, calculator.subtract(5, 3));
    }

    @Test
    @DisplayName("4 * 5 = 20")
    void multiplyTest() {
      assertEquals(20, calculator.multiply(4, 5));
    }

    @Test
    @DisplayName("10 / 2 = 5")
    void divideTest() {
      assertEquals(5, calculator.divide(10, 2));
    }

    @Test
    @DisplayName("Division by zero throws ArithmeticException")
    void divideByZeroThrowsException() {
      assertThrows(ArithmeticException.class, () -> calculator.divide(5, 0));
    }
  }

  @Nested
  @DisplayName("Complex Operations")
  class ComplexOperations {

    @Test
    @DisplayName("Complex calculation with delay")
    void complexCalculationTest() {
      int result = calculator.complexCalculation(5);
      assertEquals(35, result, "Complex calculation should work correctly");
    }

    @Test
    @DisplayName("Multiple complex calculations")
    void multipleComplexCalculations() {
      assertEquals(35, calculator.complexCalculation(5));
      assertEquals(26, calculator.complexCalculation(4));
      assertEquals(19, calculator.complexCalculation(3));
    }

    @ParameterizedTest(name = "complexCalculation({0}) = {1}")
    @CsvSource({ "1, 11", "2, 14", "3, 19", "10, 110" })
    void complexCalculationParameterized(int input, int expected) {
      assertEquals(expected, calculator.complexCalculation(input));
    }
  }
}
