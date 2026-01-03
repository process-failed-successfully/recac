package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	// Check if correct number of arguments are provided
	if len(os.Args) != 4 {
		fmt.Println("Usage: calculator <num1> <operator> <num2>")
		os.Exit(1)
	}

	// Parse the arguments
	num1, err := strconv.ParseFloat(os.Args[1], 64)
	if err != nil {
		fmt.Printf("Error: Invalid number '%s'\n", os.Args[1])
		os.Exit(1)
	}

	operator := os.Args[2]

	num2, err := strconv.ParseFloat(os.Args[3], 64)
	if err != nil {
		fmt.Printf("Error: Invalid number '%s'\n", os.Args[3])
		os.Exit(1)
	}

	// Perform the calculation based on the operator
	var result float64
	switch operator {
	case "+":
		result = num1 + num2
	case "-":
		result = num1 - num2
	case "*":
		result = num1 * num2
	case "/":
		if num2 == 0 {
			fmt.Println("Error: Division by zero")
			os.Exit(1)
		}
		result = num1 / num2
	default:
		fmt.Printf("Error: Invalid operator '%s'. Use one of: +, -, *, /\n", operator)
		os.Exit(1)
	}

	// Print the result
	fmt.Println(result)
}
