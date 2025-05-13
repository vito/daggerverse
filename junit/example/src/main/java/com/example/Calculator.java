package com.example;

/**
 * A simple calculator class to demonstrate test cases.
 */
public class Calculator {
    
    /**
     * Adds two numbers.
     * 
     * @param a first number
     * @param b second number
     * @return sum of a and b
     */
    public int add(int a, int b) {
        return a + b;
    }
    
    /**
     * Subtracts second number from first.
     * 
     * @param a first number
     * @param b second number
     * @return a minus b
     */
    public int subtract(int a, int b) {
        return a - b;
    }
    
    /**
     * Multiplies two numbers.
     * 
     * @param a first number
     * @param b second number
     * @return product of a and b
     */
    public int multiply(int a, int b) {
        return a * b;
    }
    
    /**
     * Divides first number by second.
     * 
     * @param a first number
     * @param b second number, must be non-zero
     * @return a divided by b
     * @throws ArithmeticException if b is zero
     */
    public int divide(int a, int b) {
        if (b == 0) {
            throw new ArithmeticException("Division by zero");
        }
        return a / b;
    }
    
    /**
     * Performs complex calculation with artificial delay.
     * 
     * @param input the input value
     * @return processed result after calculation
     */
    public int complexCalculation(int input) {
        try {
            // Simulate a time-consuming calculation
            Thread.sleep(1500);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
        
        return input * input + 10;
    }
}