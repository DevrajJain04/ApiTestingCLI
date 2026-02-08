package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	// "path/filepath"
)

func main() {
	runcmd := os.Args[1]
	fmt.Println("run command diya",runcmd)
	filepath := os.Args[2]
	// filepath := flag.String("f", "test.yaml", "Path to the yaml file containing the tests")
	flag.Parse()
	data ,err:= os.ReadFile(filepath) // dereferencing the string pointer

	if err!=nil{
		panic(err)
	}

	fmt.Println("Welcome to ReqRes ")
	fmt.Println("This is an API testing tool")
	fmt.Println("You write a yaml and we run the tests for you")
	fmt.Println("Prepare your yaml file and press Enter to continue...")
	fmt.Println("Check your statistics here http://localhost:8080/stats")
	fmt.Println("The yaml file path is holding ", string(data))
	
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	exitCode := scanner.Text()
	fmt.Println("You entered:", exitCode)
	if exitCode == "exit" {
		return;
	} else {
		fmt.Println("not exit wow")
	}
}
