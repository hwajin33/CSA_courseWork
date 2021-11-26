package gol

import (
	"fmt"
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

func mod(x, m int) int {
	return (x + m) % m
}

func calculateNeighbourCells(p Params, x, y int, world [][]byte) int {
	neighbours := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if !(i == 0 && j == 0){
				if world[mod(y+i, p.ImageHeight)][mod(x+j, p.ImageWidth)] == alive {
					neighbours++
				}
			}
		}
	}
	return neighbours
}


func calculateNextState(startY, endY, startX, endX int, p Params, currentTurnWorld [][]byte) [][]byte {
	newWorld := make([][]byte, endY - startY)
	for i := range newWorld {
		newWorld[i] = make([]byte, endX - startX)
	}

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			neighbours := calculateNeighbourCells(p, x, y, currentTurnWorld)
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

func calculateAliveCells(p Params, world [][]byte) []util.Cell{

	aliveCells := []util.Cell{}
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

//func getImage(p Params, c distributorChannels, im image.Image) [][]byte{
// use channels to interact with IO

//	world := make([][]byte, p.ImageHeight)
//	for i := range world {
//		world[i] = make([]byte, p.ImageWidth)
//	}
//
//	for y := 0; y < p.ImageHeight; y++ {
//		for x := 0; x < p.ImageWidth; x++ {
//			receivedImage := <- c.ioInput
//if receivedImage != 0 {
//	fmt.Println(x, y)
//}
//			world[y][x] = receivedImage
//		}
//	}
//}

func worker(startY, endY, startX, endX int, world [][]byte, out chan<- [][]uint8, p Params) {
	imagePortion := calculateNextState(startY, endY, startX, endX, p, world)
	out <- imagePortion
}

func loadImage (p Params, c distributorChannels, world [][]byte, turn int) {
	filename := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)

	c.ioCommand <- ioInput
	c.ioFilename <- filename

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	c.events <- ImageOutputComplete{turn, filename}

}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	currentWorld := make([][]byte, p.ImageHeight)
	for i := range currentWorld {
		currentWorld[i] = make([]byte, p.ImageWidth)
	}

	filename := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)

	c.ioCommand <- ioInput
	c.ioFilename <- filename

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			currentWorld[y][x] = <-c.ioInput
		}
	}

	turn := 0

	// TODO: Execute all turns of the Game of Life.
	for turn < p.Turns  {

		//nextWorld := make([][]byte, p.ImageHeight)
		//for i := range nextWorld {
		//	nextWorld[i] = make([]byte, p.ImageWidth)
		//}

		if p.Threads == 1 {
			currentWorld = calculateNextState(0, p.ImageHeight, 0, p.ImageWidth, p, currentWorld)
		} else {
			workerHeight := p.ImageHeight / p.Threads
			//modHeight := (p.ImageHeight) % p.Threads

			out := make([]chan [][]uint8, p.Threads)
			for j := range out {
				out[j] = make(chan [][]uint8)
			}

			currentThread := 0

			for currentThread < p.Threads {
				// when we reach to the last thread, start from that thread and end at the p.ImageHeight
				// if we have floating points after we get the worker height && when we reach to the last thread
				if currentThread == p.Threads - 1 {
					//fmt.Printf("t [%d] threads = %d\n", currentThread, p.Threads)
					go worker(currentThread * workerHeight, p.ImageHeight, 0, p.ImageWidth, currentWorld, out[currentThread], p)
				} else {
					//fmt.Printf("b [%d]threads = %d\n", currentThread, p.Threads)
					go worker(currentThread * workerHeight, (currentThread + 1) * workerHeight, 0, p.ImageWidth, currentWorld, out[currentThread], p)
				}
				currentThread++

				// assembling the world
				//portion := <-out[currentThread]
				//nextWorld = append(nextWorld, portion...)
				//currentWorld = nextWorld
			}

			nextWorld := make([][]byte, 0)
			// make another for loop going up #of threads -> assembling the world
			for partThread := 0; partThread < p.Threads; partThread++ {
				portion := <-out[partThread]
				nextWorld = append(nextWorld, portion...)
			}
			// swapping the worlds
			currentWorld, nextWorld = nextWorld, currentWorld
		}
		turn++
	}

	aliveCell := calculateAliveCells(p, currentWorld)
	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{turn, aliveCell}

	//loadImage(p, c, currentWorld, turn)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
