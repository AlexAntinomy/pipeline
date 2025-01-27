package main

import (
	"bufio"
	"container/ring"
	"fmt"
	"log"
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
			log.Printf("NegativeFilterStage: получено число %d", num)
			if num >= 0 {
				log.Printf("NegativeFilterStage: число %d прошло фильтр", num)
				output <- num
			} else {
				log.Printf("NegativeFilterStage: число %d отфильтровано", num)
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
			log.Printf("SpecialFilterStage: получено число %d", num)
			if num != 0 && num%3 == 0 {
				log.Printf("SpecialFilterStage: число %d прошло фильтр", num)
				output <- num
			} else {
				log.Printf("SpecialFilterStage: число %d отфильтровано", num)
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
				log.Printf("BufferStage: добавлено число %d в буфер", num)
				s.buffer.Value = num
				s.buffer = s.buffer.Next()
			case <-ticker.C:
				log.Println("BufferStage: сработал таймер, выгрузка буфера")
				s.buffer.Do(func(value interface{}) {
					if value != nil {
						log.Printf("BufferStage: выгружено число %d из буфера", value.(int))
						output <- value.(int)
					}
				})
			}
		}
	}()
	return output
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("Программа запущена")

	dataSource := func() (<-chan int, chan struct{}) {
		c := make(chan int)
		done := make(chan struct{})
		go func() {
			defer close(c)
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				input := scanner.Text()
				if strings.EqualFold(input, "exit") {
					log.Println("Программа завершила работу по команде 'exit'")
					close(done)
					return
				}
				num, err := strconv.Atoi(input)
				if err != nil {
					log.Printf("Ошибка ввода: %v", err)
					fmt.Println("Программа обрабатывает только целые числа!")
					continue
				}
				log.Printf("DataSource: получено число %d", num)
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
					log.Println("Consumer: канал закрыт, завершение работы")
					return
				}
				log.Printf("Consumer: обработаны данные: %d", num)
				fmt.Printf("Обработаны данные: %d\n", num)
			case <-done:
				log.Println("Consumer: получен сигнал завершения, завершение работы")
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
	log.Println("Программа завершена")
}
