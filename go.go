package main

import (
	"bufio"
	"container/ring"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	bufferDrainInterval = 5 * time.Second
	bufferSize          = 10
)

type Stage interface {
	Process(<-chan int) <-chan int
}

type NegativeFilterStage struct{}

func (s *NegativeFilterStage) Process(input <-chan int) <-chan int {
	output := make(chan int)
	go func() {
		defer close(output)
		for num := range input {
			if num >= 0 {
				output <- num
			}
		}
	}()
	return output
}

type SpecialFilterStage struct{}

func (s *SpecialFilterStage) Process(input <-chan int) <-chan int {
	output := make(chan int)
	go func() {
		defer close(output)
		for num := range input {
			if num != 0 && num%3 == 0 {
				output <- num
			}
		}
	}()
	return output
}

type BufferStage struct {
	buffer *ring.Ring
}

func NewBufferStage(size int) *BufferStage {
	return &BufferStage{buffer: ring.New(size)}
}

func (s *BufferStage) Process(input <-chan int) <-chan int {
	output := make(chan int)
	go func() {
		defer close(output)
		ticker := time.NewTicker(bufferDrainInterval)
		defer ticker.Stop()
		for {
			select {
			case num, ok := <-input:
				if !ok {
					return
				}
				s.buffer.Value = num
				s.buffer = s.buffer.Next()
			case <-ticker.C:
				s.buffer.Do(func(value interface{}) {
					if value != nil {
						output <- value.(int)
					}
				})
			}
		}
	}()
	return output
}

func main() {
	dataSource := func() (<-chan int, chan struct{}) {
		c := make(chan int)
		done := make(chan struct{})
		go func() {
			defer close(c)
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				input := scanner.Text()
				if strings.EqualFold(input, "exit") {
					fmt.Println("Программа завершила работу!")
					close(done)
					return
				}
				num, err := strconv.Atoi(input)
				if err != nil {
					fmt.Println("Программа обрабатывает только целые числа!")
					continue
				}
				c <- num
			}
		}()
		return c, done
	}

	consumer := func(input <-chan int, done chan struct{}) {
		for {
			select {
			case num, ok := <-input:
				if !ok {
					return
				}
				fmt.Printf("Обработаны данные: %d\n", num)
			case <-done:
				return
			}
		}
	}

	source, done := dataSource()
	stages := []Stage{
		&NegativeFilterStage{},
		&SpecialFilterStage{},
		NewBufferStage(bufferSize),
	}

	var pipeline <-chan int = source
	for _, stage := range stages {
		pipeline = stage.Process(pipeline)
	}

	consumer(pipeline, done)
}
