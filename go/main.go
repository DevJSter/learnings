package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Recipe represents a cooking task
type Recipe struct {
	ID         int
	Name       string
	Difficulty int // 1-5 scale
	Result     string
}

func main() {
	fmt.Println("🍳 Advanced Gopher Kitchen Simulation")
	fmt.Println("=====================================")

	// Example 1: Basic goroutines with channels
	fmt.Println("\n1. Basic Goroutines with Unbuffered Channel:")
	basicGoroutines()

	// Example 2: Buffered channels
	fmt.Println("\n2. Buffered Channels (Non-blocking):")
	bufferedChannels()

	// Example 3: Worker pool pattern
	fmt.Println("\n3. Worker Pool Pattern:")
	workerPool()

	// Example 4: Channel directions and closing
	fmt.Println("\n4. Channel Directions & Closing:")
	channelDirections()

	// Example 5: Select statement for multiple channels
	fmt.Println("\n5. Select Statement (Multiple Channels):")
	selectExample()
}

// Example 1: Basic goroutines (your original code enhanced)
func basicGoroutines() {
	c := make(chan int) // Unbuffered channel - synchronous

	for i := 0; i < 3; i++ {
		go cookingGopher(i, c)
	}

	for i := 0; i < 3; i++ {
		gopherID := <-c // This BLOCKS until a goroutine sends
		fmt.Printf("   ✅ Gopher %d finished cooking\n", gopherID)
	}
}

func cookingGopher(id int, c chan int) {
	cookTime := time.Duration(rand.Intn(1000)) * time.Millisecond
	fmt.Printf("   🧑‍🍳 Gopher %d started cooking (will take %v)\n", id, cookTime)

	time.Sleep(cookTime) // Simulate cooking time
	c <- id              // Send result back
}

// Example 2: Buffered channels - can hold values without blocking
func bufferedChannels() {
	// Buffered channel can hold 3 values before blocking
	orders := make(chan string, 3)

	// Send to buffered channel (won't block until buffer is full)
	orders <- "🍕 Pizza"
	orders <- "🍔 Burger"
	orders <- "🍜 Ramen"
	fmt.Println("   📝 All orders placed in buffer (non-blocking)")

	// Receive from channel
	for i := 0; i < 3; i++ {
		order := <-orders
		fmt.Printf("   📋 Processing order: %s\n", order)
	}
}

// Example 3: Worker Pool - limited number of workers processing many jobs
func workerPool() {
	const numWorkers = 2
	const numJobs = 5

	jobs := make(chan Recipe, numJobs)
	results := make(chan string, numJobs)

	// Start workers
	var wg sync.WaitGroup
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg)
	}

	// Send jobs
	recipes := []Recipe{
		{1, "Pasta", 2, ""},
		{2, "Steak", 4, ""},
		{3, "Salad", 1, ""},
		{4, "Cake", 5, ""},
		{5, "Soup", 3, ""},
	}

	for _, recipe := range recipes {
		jobs <- recipe
	}
	close(jobs) // Important: close channel to signal no more jobs

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(results) // Close results channel when all done
	}()

	// Collect results
	for result := range results { // range over channel until closed
		fmt.Printf("   %s\n", result)
	}
}

func worker(id int, jobs <-chan Recipe, results chan<- string, wg *sync.WaitGroup) {
	defer wg.Done() // Ensure Done is called when function exits

	for recipe := range jobs { // Receive jobs until channel is closed
		cookTime := time.Duration(recipe.Difficulty*200) * time.Millisecond
		fmt.Printf("   👨‍🍳 Worker %d cooking %s (difficulty: %d)\n",
			id, recipe.Name, recipe.Difficulty)

		time.Sleep(cookTime)
		result := fmt.Sprintf("✨ Worker %d completed %s", id, recipe.Name)
		results <- result
	}
}

// Example 4: Channel directions (send-only, receive-only) and closing
func channelDirections() {
	messages := make(chan string, 2)

	// Send-only channel parameter
	go sender(messages)

	// Receive-only channel parameter
	go receiver(messages)

	time.Sleep(500 * time.Millisecond)
}

func sender(ch chan<- string) { // send-only channel
	ch <- "🎵 Hello"
	ch <- "🎶 World"
	close(ch) // Close channel when done sending
}

func receiver(ch <-chan string) { // receive-only channel
	for msg := range ch { // range automatically handles channel closing
		fmt.Printf("   📨 Received: %s\n", msg)
	}
	fmt.Println("   🔚 Channel closed, receiver done")
}

// Example 5: Select statement - non-blocking channel operations
func selectExample() {
	c1 := make(chan string)
	c2 := make(chan string)

	// Two goroutines sending at different times
	go func() {
		time.Sleep(200 * time.Millisecond)
		c1 <- "🥗 Fast food ready"
	}()

	go func() {
		time.Sleep(400 * time.Millisecond)
		c2 <- "🥘 Slow food ready"
	}()

	// Select waits for first available channel
	for i := 0; i < 2; i++ {
		select {
		case msg1 := <-c1:
			fmt.Printf("   ⚡ %s\n", msg1)
		case msg2 := <-c2:
			fmt.Printf("   🐌 %s\n", msg2)
		case <-time.After(100 * time.Millisecond):
			fmt.Println("   ⏰ Timeout waiting for food...")
		}
	}
}
