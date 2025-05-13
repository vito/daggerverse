package com.example;

/**
 * Utility class for string operations.
 */
public class StringUtils {
    
    /**
     * Reverses a string.
     * 
     * @param input string to reverse
     * @return reversed string
     */
    public static String reverse(String input) {
        if (input == null) {
            return null;
        }
        return new StringBuilder(input).reverse().toString();
    }
    
    /**
     * Checks if a string is a palindrome (reads the same forward and backward).
     * 
     * @param input string to check
     * @return true if palindrome, false otherwise
     */
    public static boolean isPalindrome(String input) {
        if (input == null) {
            return false;
        }
        String cleaned = input.toLowerCase().replaceAll("[^a-z0-9]", "");
        return cleaned.equals(new StringBuilder(cleaned).reverse().toString());
    }
    
    /**
     * Processes a string with artificial delay to simulate complex operation.
     * 
     * @param input string to process
     * @return processed string
     */
    public static String processWithDelay(String input) {
        try {
            // Simulate a time-consuming operation
            Thread.sleep(2000);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
        
        if (input == null || input.isEmpty()) {
            return "";
        }
        
        return input.toUpperCase() + "-PROCESSED";
    }
}