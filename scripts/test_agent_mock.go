package main

import (
	"context"
	"fmt"
	"recac/internal/agent"
)

func main() {
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("custom response")
	res, _ := mockAgent.Send(context.Background(), "hello")
	fmt.Println(res)
}
