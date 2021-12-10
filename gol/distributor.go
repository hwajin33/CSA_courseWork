package gol

import (
	"fmt"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

const alive = 255
const dead = 0

func getAliveNeighbour(p Params, x, y int, world [][]byte) int {
	neighbours := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if !(i == 0 && j == 0) {
				if world[modulus(y+i, p.ImageHeight)][modulus(x+j, p.ImageWidth)] == alive {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func modulus(i, m int) int {
	return (i + m) % m
}

func calculateNextState(startY, endY, startX, endX int, p Params, currentTurnWorld [][]byte) [][]byte {
	newWorld := make([][]byte, endY-startY)
	for i := range newWorld {
		newWorld[i] = make([]byte, endX-startX)
	}

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			neighbours := getAliveNeighbour(p, x, y, currentTurnWorld)
			if currentTurnWorld[y][x] == alive {
				if neighbours == 2 || neighbours == 3 {
					newWorld[y-startY][x] = alive
				} else {
					newWorld[y-startY][x] = dead
				}
			} else {
				if neighbours == 3 {
					newWorld[y-startY][x] = alive
				} else {
					newWorld[y-startY][x] = dead
				}
			}
		}
	}
	return newWorld
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {

	var aliveCells []util.Cell
	var cell util.Cell

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				cell.X = x
				cell.Y = y
				aliveCells = append(aliveCells, cell)
			}
		}
	}
	return aliveCells
}

func worker(startY, endY, startX, endX int, world [][]byte, out chan<- [][]uint8, p Params) {
	imagePortion := calculateNextState(startY, endY, startX, endX, p, world)
	out <- imagePortion
}

func saveImage(p Params, c distributorChannels, world [][]byte, turn int) {
	filename := fmt.Sprintf("%vx%vx%d", p.ImageHeight, p.ImageWidth, turn)

	c.ioCommand <- ioOutput
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	// iterate through every single cell in current world and everytime there's an alive cell pass a cell flipped event through the events channel
	turn := 0

	c.events <- StateChange{turn, Executing}

	// TODO: Create a 2D slice to store the world.
	currentWorld := make([][]byte, p.ImageHeight)
	for i := range currentWorld {
		currentWorld[i] = make([]byte, p.ImageWidth)
	}

	filename := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)

	c.ioCommand <- ioInput
	c.ioFilename <- filename

	// loading the image from the IO
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			currentWorld[y][x] = <-c.ioInput
		}
	}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if currentWorld [y][x] == alive {
				cell := util.Cell{X: x, Y: y}
				c.events <- CellFlipped{CompletedTurns: 0, Cell: cell}
			}
		}
	}

	//make aliveCellCount inside the ticker
	ticker := time.NewTicker(2 * time.Second)
	mutex := new(sync.Mutex)
	done := make(chan bool)

	go func() {
		for  { //turn < p.Turns
			select {
			//key presses
			case buttonKey := <- keyPresses:
				if buttonKey == 'q' {
					saveImage(p, c, currentWorld, turn)
					c.events <- ImageOutputComplete{CompletedTurns: turn}
					c.events <- FinalTurnComplete{CompletedTurns: turn}
					c.events <- StateChange{turn, Quitting}
				} else if buttonKey == 's' {
					saveImage(p, c, currentWorld, turn)
					c.events <- ImageOutputComplete{CompletedTurns: turn}
				} else if buttonKey == 'p' {
					mutex.Lock()
					c.events <- StateChange{turn, Paused}
					for {
						pAgain := <- keyPresses
						if pAgain == 'p' {
							fmt.Println("Continuing")
							c.events <- StateChange{turn, Executing}
							mutex.Unlock()
							break
						}
					}
				}

			// ticker
			case <-done:
				return
			case <-ticker.C:
				mutex.Lock()
				t:= turn
				aliveCells:= len(calculateAliveCells(p, currentWorld))
				mutex.Unlock()
				c.events <- AliveCellsCount{CompletedTurns: t, CellsCount: aliveCells}
			default:
			}
		}
	}()

	// TODO: Execute all turns of the Game of Life.
	for turn < p.Turns {
		if p.Threads == 1 {
			currentWorld = calculateNextState(0, p.ImageHeight, 0, p.ImageWidth, p, currentWorld)
		} else {
			workerHeight := p.ImageHeight / p.Threads

			out := make([]chan [][]uint8, p.Threads)
			for j := range out {
				out[j] = make(chan [][]uint8)
			}

			currentThread := 0

			for currentThread < p.Threads {
				// when we reach to the last thread, start from that thread and end at the p.ImageHeight
				if currentThread == p.Threads - 1 {
					go worker(currentThread*workerHeight, p.ImageHeight, 0, p.ImageWidth, currentWorld, out[currentThread], p)
				} else {
					go worker(currentThread*workerHeight, (currentThread+1)*workerHeight, 0, p.ImageWidth, currentWorld, out[currentThread], p)
				}
				currentThread++
			}

			nextWorld := make([][]byte, 0)
			// assembling the world
			for partThread := 0; partThread < p.Threads; partThread++ {
				portion := <-out[partThread]
				nextWorld = append(nextWorld, portion...)
			}

			for y := 0; y < p.ImageHeight; y++ {
				for x := 0; x < p.ImageWidth; x++ {
					if nextWorld [y][x] != currentWorld[y][x] {
						cell := util.Cell{X: x, Y: y}
						c.events <- CellFlipped{CompletedTurns: turn, Cell: cell}
					}
				}
			}

			mutex.Lock()
			// swapping the worlds
			currentWorld, nextWorld = nextWorld, currentWorld
			mutex.Unlock()
		}
		mutex.Lock()
		turn++
		mutex.Unlock()

		c.events <- TurnComplete{turn}
	}

	done <- true

	saveImage(p, c, currentWorld, turn)

	aliveCell := calculateAliveCells(p, currentWorld)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{turn, aliveCell}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}