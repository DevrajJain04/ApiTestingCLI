package main

import (
	"fmt"
	"bufio"
	"os"
)

func main() {
	// print("Welcome to ReqRes")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	// text := scanner.Text()
	nums := scanner.Bytes()
	// convert nums to integer
	num := 0
	for _, b := range nums {
		if b >= '0' && b <= '9' {
			num = num*10 + int(b-'0')
		}
	};
	x := num+1
	fmt.Print("your input was ",num,x)

}
