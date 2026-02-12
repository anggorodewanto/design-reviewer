package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("server running on :8080")
	http.ListenAndServe(":8080", nil)
}
