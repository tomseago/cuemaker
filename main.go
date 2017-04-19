package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

type ParseState int

const (
	ROOT ParseState = iota
	DJ_PLAYLISTS
	COLLECTION
	TRACK
)

var state = ROOT

var dec *xml.Decoder
var enc *xml.Encoder

var tracksChanged = 0
var cuesAdded = 0

func main() {

	useDefaults := flag.Bool("default", false, "Use default filenames")

	inplaceRename := flag.Bool("rename", false, "Do in place rename of input file")

	inputFilename := flag.String("in", "rekordbox.xml", "Input filename")
	outputFilename := flag.String("out", "/tmp/output.xml", "Temporary output filename")

	flag.Parse()

	if *useDefaults {
		usr, _ := user.Current()
		dir := usr.HomeDir

		*inputFilename = filepath.Join(dir, "Documents", "rekordbox.xml")
		*outputFilename = filepath.Join(dir, "Library", "Pioneer", "rekordbox", "rekordbox.xml")

		fmt.Printf("\nUsing Standard Default Values\n")
	}

	fmt.Printf("Input : %s\n", *inputFilename)
	fmt.Printf("Output: %s\n", *outputFilename)

	fmt.Printf("\nPress enter to continue or CTRL-C to stop...")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	fin, err := os.Open(*inputFilename)
	if err != nil {
		fmt.Printf("Can't open input file '%v': %v\n", *inputFilename, err)
		os.Exit(1)
	}

	fout, err := os.Create(*outputFilename)
	if err != nil {
		fmt.Printf("Can't open output file '%v': %v\n", *outputFilename, err)
		os.Exit(2)
	}

	dec = xml.NewDecoder(fin)
	enc = xml.NewEncoder(fout)

	for tok, err := dec.Token(); tok != nil; tok, err = dec.Token() {
		if err != nil {
			fmt.Printf("Parser error: %v\n", err)
			os.Exit(10)
		}

		switch state {
		case ROOT:
			parseAtRoot(tok)

		case DJ_PLAYLISTS:
			parseAtDJPlaylists(tok)

		case COLLECTION:
			parseAtCollection(tok)

		case TRACK:
			parseAtTrack(tok)
		}
	}

	err = enc.Flush()
	if err != nil {
		fmt.Printf("Error %v\n")
		os.Exit(11)
	}

	fout.Close()
	fin.Close()

	// Now copy the output to the input
	if *inplaceRename {
		os.Remove(*inputFilename + ".bak")
		os.Rename(*inputFilename, *inputFilename+".bak")
		os.Rename(*outputFilename, *inputFilename)
	}

	fmt.Printf("Finished  %v tracks changed, %v cues added\n", tracksChanged, cuesAdded)
}

func parseAtRoot(tok xml.Token) {
	enc.EncodeToken(tok)

	switch el := tok.(type) {
	case xml.StartElement:
		if el.Name.Local == "DJ_PLAYLISTS" {
			state = DJ_PLAYLISTS
		}

	case xml.EndElement:

	}
}

func parseAtDJPlaylists(tok xml.Token) {
	enc.EncodeToken(tok)

	switch el := tok.(type) {
	case xml.StartElement:
		if el.Name.Local == "COLLECTION" {
			fmt.Printf("Found collection\n")
			state = COLLECTION
		}

	case xml.EndElement:
		if el.Name.Local == "DJ_PLAYLISTS" {
			state = ROOT
		}
	}

}

var trackName string

type cuePoint struct {
	hotCue bool
	cue    bool
}

var cues map[string]cuePoint

func parseAtCollection(tok xml.Token) {
	enc.EncodeToken(tok)

	switch el := tok.(type) {
	case xml.StartElement:
		if el.Name.Local == "TRACK" {
			trackName = ""
			for _, attr := range el.Attr {
				if attr.Name.Local == "Name" {
					trackName = attr.Value
				}
			}
			cues = make(map[string]cuePoint)

			// fmt.Printf("Found track %v\n", trackName)
			state = TRACK
		}

	case xml.EndElement:
		if el.Name.Local == "COLLECTION" {
			state = DJ_PLAYLISTS
		}
	}

}

func parseAtTrack(tok xml.Token) {

	switch el := tok.(type) {
	case xml.StartElement:
		enc.EncodeToken(tok)
		if el.Name.Local == "POSITION_MARK" {
			start := ""
			num := ""
			for _, attr := range el.Attr {
				switch attr.Name.Local {
				case "Start":
					start = attr.Value

				case "Num":
					num = attr.Value
				}
			}

			// Record either a hotcue or a cue for this start
			if start != "" && num != "" {
				cue, ok := cues[start]
				if !ok {
					cue = cuePoint{false, false}
					cues[start] = cue
				}
				if num == "-1" {
					cue.cue = true
				} else {
					cue.hotCue = true
				}
				// fmt.Printf("%v: %v\n", start, cue)
				cues[start] = cue
			} else {
				fmt.Printf("Didn't understand: %v\n", el)
			}
		}

	case xml.EndElement:
		if el.Name.Local == "TRACK" {
			// For everything that has a hotcue but not a cue, we are going to
			// add a new cue

			// fmt.Printf("%v\n", trackName)
			// fmt.Printf("%v\n", cues)

			trackChanged := false
			for start, cue := range cues {
				if cue.hotCue && !cue.cue {
					el := xml.StartElement{
						Name: xml.Name{Local: "POSITION_MARK"},
						Attr: make([]xml.Attr, 0),
					}
					el.Attr = append(el.Attr, xml.Attr{Name: xml.Name{Local: "Name"}, Value: ""})
					el.Attr = append(el.Attr, xml.Attr{Name: xml.Name{Local: "Type"}, Value: "0"})
					el.Attr = append(el.Attr, xml.Attr{Name: xml.Name{Local: "Start"}, Value: start})
					el.Attr = append(el.Attr, xml.Attr{Name: xml.Name{Local: "Num"}, Value: "-1"})

					enc.EncodeToken(el)
					enc.EncodeToken(el.End())

					fmt.Printf("%v : Adding cue at %v\n", trackName, start)

					if !trackChanged {
						trackChanged = true
						tracksChanged++
					}
					cuesAdded++
				}
			}

			// Can't write the end until we've finished adding the things above
			enc.EncodeToken(tok)
			// enc.EncodeToken(tok.End())

			// Back to collection state
			state = COLLECTION
		} else {
			enc.EncodeToken(tok)
		}

	default:
		// For other things
		enc.EncodeToken(tok)

	}
}

func parseAtPlaylists(tok xml.Token) {

}
