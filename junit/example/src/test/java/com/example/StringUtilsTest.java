package com.example;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.CsvSource;
import org.junit.jupiter.params.provider.MethodSource;
import org.junit.jupiter.params.provider.NullAndEmptySource;

import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;

@DisplayName("String Utilities Tests")
class StringUtilsTest {
    
    @Nested
    @DisplayName("String Reversal")
    class ReverseTests {
        
        @Test
        @DisplayName("Reverse 'hello' gives 'olleh'")
        void reverseSimpleString() {
            assertEquals("olleh", StringUtils.reverse("hello"));
        }
        
        @Test
        @DisplayName("Reverse empty string")
        void reverseEmptyString() {
            assertEquals("", StringUtils.reverse(""));
        }
        
        @Test
        @DisplayName("Reverse null gives null")
        void reverseNullString() {
            assertNull(StringUtils.reverse(null));
        }
        
        @ParameterizedTest(name = "reverse({0}) = {1}")
        @CsvSource({
            "abc, cba",
            "test, tset",
            "A, A",
            "'', ''",
            "12345, 54321"
        })
        void reverseDifferentStrings(String input, String expected) {
            assertEquals(expected, StringUtils.reverse(input));
        }
    }
    
    @Nested
    @DisplayName("Palindrome Checks")
    static class PalindromeTests {
        
        @Test
        @DisplayName("'radar' is a palindrome")
        void simpleWordPalindrome() {
            assertTrue(StringUtils.isPalindrome("radar"));
        }
        
        @Test
        @DisplayName("'hello' is not a palindrome")
        void nonPalindrome() {
            assertFalse(StringUtils.isPalindrome("hello"));
        }
        
        @Test
        @DisplayName("'A man, a plan, a canal: Panama' is a palindrome")
        void complexPalindrome() {
            assertTrue(StringUtils.isPalindrome("A man, a plan, a canal: Panama"));
        }
        
        @ParameterizedTest(name = "{0} is a palindrome")
        @MethodSource("providePalindromes")
        void variousPalindromes(String input) {
            assertTrue(StringUtils.isPalindrome(input));
        }
        
        static Stream<Arguments> providePalindromes() {
            return Stream.of(
                Arguments.of("racecar"),
                Arguments.of("level"),
                Arguments.of("Madam, I'm Adam"),
                Arguments.of("10801"),
                Arguments.of("No 'x' in Nixon")
            );
        }
        
        @ParameterizedTest
        @NullAndEmptySource
        void nullAndEmptyAreNotPalindromes(String input) {
            assertFalse(StringUtils.isPalindrome(input));
        }
    }
    
    @Nested
    @DisplayName("Delayed Processing")
    class DelayedProcessingTests {
        
        @Test
        @DisplayName("Process with delay - simple string")
        void processWithDelaySimpleTest() {
            String result = StringUtils.processWithDelay("hello");
            assertEquals("HELLO-PROCESSED", result);
        }
        
        @Test
        @DisplayName("Process with delay - empty string")
        void processWithDelayEmptyString() {
            String result = StringUtils.processWithDelay("");
            assertEquals("", result);
        }
        
        @Test
        @DisplayName("Process with delay - null string")
        void processWithDelayNullString() {
            String result = StringUtils.processWithDelay(null);
            assertEquals("", result);
        }
        
        @ParameterizedTest(name = "processWithDelay({0}) includes original text")
        @CsvSource({
            "test",
            "example",
            "another"
        })
        void processWithDelayVariousStrings(String input) {
            String result = StringUtils.processWithDelay(input);
            assertEquals(input.toUpperCase() + "-PROCESSED", result);
        }
    }
}