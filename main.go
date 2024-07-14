package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/in"
	"xabbo.b7c.io/goearth/shockwave/out"
	"xabbo.b7c.io/goearth/shockwave/profile"
	"xabbo.b7c.io/goearth/shockwave/room"
)

var ext = g.NewExt(g.ExtInfo{
	Title:       "autogarland",
	Description: "An extension to automatically outline your room using garlands.",
	Author:      "chirp24",
	Version:     "1.0",
})

var roomMgr = room.NewManager(ext)
var profileMgr = profile.NewManager(ext)

type FurniMover struct {
	ext     *g.Ext
	furniID string
	pos     struct {
		W1, W2, L1, L2 int
		Direction      string
	}
	log                 []string
	logMu               sync.Mutex
	packetStr           string
	captureMode         bool
	captureMu           sync.Mutex
	autoMode            bool
	predefinedPositions []struct{ L1, L2, W1, W2 int }
	placedPositions     map[string]struct{}
	positionIndex       int
}

func NewFurniMover(ext *g.Ext) *FurniMover {
	return &FurniMover{
		ext:             ext,
		placedPositions: make(map[string]struct{}),
		positionIndex:   0,
		predefinedPositions: []struct{ L1, L2, W1, W2 int }{
			{L1: 584, L2: 104, W1: 3, W2: 11},
			{L1: 432, L2: 181, W1: 3, W2: 8},
			{L1: 216, L2: 289, W1: 3, W2: 3},
			{L1: 159, L2: 318, W1: 3, W2: 3},
			{L1: 136, L2: 330, W1: 3, W2: 4},
			{L1: 80, L2: 358, W1: 3, W2: 4},
			{L1: -8, L2: 401, W1: 3, W2: 3},
			{L1: -4, L2: 400, W1: 3, W2: 4},
			{L1: -349, L2: 367, W1: 9, W2: 0},
			{L1: -402, L2: 341, W1: 9, W2: 0},
			{L1: -458, L2: 313, W1: 9, W2: 0},
			{L1: -511, L2: 286, W1: 9, W2: 0},
			{L1: -558, L2: 263, W1: 9, W2: 0},
		},
	}
}

func (fm *FurniMover) setupExt() {
	fm.ext.Intercept(out.CHAT).With(fm.handleTalk)
	fm.ext.Intercept(out.SHOUT).With(fm.handleTalk)
	fm.ext.Intercept(in.ITEMS_2).With(fm.handleItems2)
	fm.ext.Intercept(in.UPDATEITEM).With(fm.handleUpdateItem)
	fm.ext.Intercept(out.ADDSTRIPITEM).With(fm.handleAddStripItem)
}

func (fm *FurniMover) runExt() {
	defer os.Exit(0)
	fm.ext.Run()
}

func (fm *FurniMover) AddLogMsg(msg string) {
	fm.logMu.Lock()
	defer fm.logMu.Unlock()
	fm.log = append(fm.log, msg)
	if len(fm.log) > 100 {
		fm.log = fm.log[1:]
	}
	fmt.Println(strings.Join(fm.log, "\n"))
}

func showMsg(msg string) {
	self := roomMgr.EntityByName(profileMgr.Name)
	if self == nil {
		fmt.Println("self not found.")
		return
	}
	ext.Send(in.CHAT, self.Index, msg)
}

func (fm *FurniMover) handleTalk(e *g.Intercept) {
	msg := e.Packet.ReadString()

	// Check if auto mode is enabled
	if fm.autoMode {
		// Process player input when auto mode is enabled
		switch msg {
		case "Y":
			// Player chooses to continue placing garlands
			e.Block()
			if fm.positionIndex >= len(fm.predefinedPositions) {
				go showMsg("> > Please place your garland on the left wall, and then right. < <")
				fm.positionIndex = len(fm.predefinedPositions) // Skip predefined positions
				fm.predefinedPositions = append(fm.predefinedPositions,
					struct{ L1, L2, W1, W2 int }{
						L1: 485, L2: 248, W1: 3, W2: 9,
					},
					struct{ L1, L2, W1, W2 int }{
						L1: 93, L2: 296, W1: 9, W2: 0,
					},
				)
			}
		case "N":
			// Player chooses to end auto mode
			fm.autoMode = false
			e.Block()
			go showMsg("> > Auto mode completed. Automode turning off... < <")
		default:
			// Ignore other messages when in auto mode
		}
	} else {
		// Process other commands when auto mode is not enabled
		if msg == ":auto" {
			// Toggle auto mode
			fm.autoMode = !fm.autoMode
			if fm.autoMode {
				go showMsg("> > Auto mode enabled. Please place your garlands. < <")
			} else {
				go showMsg("> > Auto mode disabled. < <")
			}
			e.Block()
		}
	}
}

func (fm *FurniMover) handleItems2(e *g.Intercept) {
	if fm.autoMode {
		fm.packetStr = e.Packet.ReadString()
		fm.handleItemPacket(fm.packetStr, "Item placed")
	}
}

func (fm *FurniMover) handleUpdateItem(e *g.Intercept) {
	if fm.autoMode {
		fm.packetStr = e.Packet.ReadString()
		fm.handleItemPacket(fm.packetStr, "Item updated")
	}
}

func (fm *FurniMover) handleItemPacket(packetStr, actionType string) {
	parts := strings.Split(packetStr, "\t")
	if len(parts) < 5 {
		return
	}
	newID := parts[0]
	pos := parts[3]
	posParts := strings.Fields(pos)
	var newPos struct {
		W1, W2, L1, L2 int
		Direction      string
	}
	for _, part := range posParts {
		if strings.HasPrefix(part, ":w=") {
			wParts := strings.Split(strings.TrimPrefix(part, ":w="), ",")
			if len(wParts) == 2 {
				newPos.W1, _ = strconv.Atoi(wParts[0])
				newPos.W2, _ = strconv.Atoi(wParts[1])
			}
		} else if strings.HasPrefix(part, "l=") {
			lParts := strings.Split(strings.TrimPrefix(part, "l="), ",")
			if len(lParts) == 2 {
				newPos.L1, _ = strconv.Atoi(lParts[0])
				newPos.L2, _ = strconv.Atoi(lParts[1])
			}
		} else if part == "r" || part == "l" {
			newPos.Direction = part
		}
	}

	if newID != fm.furniID || newPos != fm.pos {
		fm.furniID = newID
		fm.pos = newPos
		fm.AddLogMsg(fmt.Sprintf("%s: ID %s, Position: w=%d,%d l=%d,%d %s",
			actionType, fm.furniID, fm.pos.W1, fm.pos.W2, fm.pos.L1, fm.pos.L2, fm.pos.Direction))
		fm.moveToNextPredefinedPosition()
	}
}

func (fm *FurniMover) handleAddStripItem(e *g.Intercept) {
	fm.captureMu.Lock()
	if fm.captureMode {
		fm.captureMode = false
		fm.captureMu.Unlock()
		packetContent := e.Packet.ReadString()
		parts := strings.Split(packetContent, " ")
		if len(parts) >= 3 {
			fm.furniID = parts[2]
			fm.AddLogMsg(fmt.Sprintf("Captured furni ID: %s", fm.furniID))
		}
		return
	}
	fm.captureMu.Unlock()
}

func (fm *FurniMover) moveToNextPredefinedPosition() {
	if fm.positionIndex >= len(fm.predefinedPositions) {
		fm.AddLogMsg("No more predefined positions available.")
		// Prompt player to place garlands on selector box
		go showMsg(" > > Place garlands on selector box? Y/N < <")
		return
	}

	nextPos := fm.predefinedPositions[fm.positionIndex]
	positionKey := fmt.Sprintf("%d,%d,%d,%d", nextPos.L1, nextPos.L2, nextPos.W1, nextPos.W2)

	if _, exists := fm.placedPositions[positionKey]; exists {
		fm.positionIndex++
		fm.moveToNextPredefinedPosition()
		return
	}

	fm.placedPositions[positionKey] = struct{}{}
	fm.pos.L1 = nextPos.L1
	fm.pos.L2 = nextPos.L2
	fm.pos.W1 = nextPos.W1
	fm.pos.W2 = nextPos.W2
	fm.moveWallItem()

	fm.positionIndex++
}

func (fm *FurniMover) moveWallItem() {
	placestuffData := fmt.Sprintf(":w=%d,%d l=%d,%d %s", fm.pos.W1, fm.pos.W2, fm.pos.L1, fm.pos.L2, fm.pos.Direction)
	numFurniID, _ := strconv.Atoi(fm.furniID)
	fm.ext.Send(g.Out.Id("MOVEITEM"), numFurniID, placestuffData)
	fm.AddLogMsg(fmt.Sprintf("Moved item ID %s to predefined position: w=%d,%d l=%d,%d %s", fm.furniID, fm.pos.W1, fm.pos.W2, fm.pos.L1, fm.pos.L2, fm.pos.Direction))
}

func main() {
	furniMover := NewFurniMover(ext)

	furniMover.setupExt()
	furniMover.runExt()
}
