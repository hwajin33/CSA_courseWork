package gol

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func calculateNextState(p Params, world [][]byte) [][]byte {
	iWidth := p.ImageWidth
	iHeight := p.ImageHeight
	r := iWidth
	c := iHeight

	makeAnotherWorld := make([][]byte, len(world))
	for i := range world {
		makeAnotherWorld[i] = make([]byte, len(world[i]))
		copy(makeAnotherWorld[i], world[i])
	}

	for imageWidth := 0; imageWidth < iWidth; imageWidth++ {

		//if condition --> cal state
		// cal. live cell -> array send the live ones
		// for loop -> if alive -> print

		for imageHeight := 0; imageHeight < iHeight; imageHeight++ {
			// 2 more for loop -> conditions for cal
			// if exclude itself (row = next step)

			alive := 0
			for i := r - 1; i <= r + 1; i++ {
				for k := c - 1; k <= c + 1; k++ {

					if i == r && k == c {
						continue
					}
					if makeAnotherWorld[((iWidth + i) % iWidth)] [((iHeight + k) % iHeight)] == 255 {
						alive++
					}
				}
			}

			if alive < 2 || alive > 3 {
				world[r][c] = 0
			}
			if alive == 3 {
				world[r][c] = 255
			}
		}
	}
	return world
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.

	turn := 0

	//for i := 0; i < p.Turns; i++ {}

	// TODO: Execute all turns of the Game of Life.

	// TODO: Report the final state using FinalTurnCompleteEvent.



	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
