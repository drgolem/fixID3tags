package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/karrick/godirwalk"
	"golang.org/x/text/encoding/charmap"
)

type ChangeAction int

const (
	ChangeAction_Halt ChangeAction = iota
	ChangeAction_Skip
	ChangeAction_Owerride
)

const (
	idTagTitle  = "Title"
	idTagArtist = "Artist"
	idTagAlbum  = "Album/Movie/Show title"
)

type ChangeStrategy struct {
	changeAction  ChangeAction
	overrideValue string
}

func main() {
	fmt.Println("tag fix")

	dryRun := flag.Bool("dry-run", true, "dry run, no changes")

	flag.Parse()

	haltOnEmptyAlbum := true

	changeActions := map[string]ChangeStrategy{
		idTagArtist: {
			ChangeAction_Owerride,
			"Вахтанг Кикабидзе",
		},
		idTagAlbum: {
			ChangeAction_Owerride,
			"Вахтанг Кикабидзе",
		},
	}

	if haltOnEmptyAlbum {
		changeActions[idTagAlbum] = ChangeStrategy{
			changeAction: ChangeAction_Halt,
		}
	}

	//dirname := "/Users/val/Music/test"
	dirname := "/Users/val/Music/Вахтанг Кикабидзе"
	//dirname := "/Users/val/Music/Марина Капуро(дискография)/1980 - Рок-группа Яблоко(КА90-14435-6)"

	updateCount := 0

	err := godirwalk.Walk(dirname, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {

			if strings.Contains(osPathname, ".git") {
				return godirwalk.SkipThis
			}
			//fmt.Printf("%s %s\n", de.ModeType(), osPathname)

			if !de.IsRegular() {
				return nil
			}

			ext := filepath.Ext(de.Name())
			if ext != ".mp3" {
				return nil
			}

			tag, err := id3v2.Open(osPathname,
				id3v2.Options{
					Parse:       true,
					ParseFrames: []string{idTagArtist, idTagAlbum, idTagTitle},
				})
			if err != nil {
				log.Fatalf("Error while opening mp3 file [%s]: %v", osPathname, err)
			}
			defer tag.Close()

			artist := tag.GetTextFrame(tag.CommonID(idTagArtist))
			album := tag.GetTextFrame(tag.CommonID(idTagAlbum))
			title := tag.GetTextFrame(tag.CommonID(idTagTitle))

			valueUpdaterFun := func(idTag string, tagFrame id3v2.TextFrame, updaterFn func(value string)) bool {
				updated := false

				val := tagFrame.Text

				if len(val) == 0 {
					if changeActions[idTag].changeAction == ChangeAction_Owerride {
						newVal := changeActions[idTag].overrideValue
						fmt.Printf("EMPTY [%s] (file: %s), set to [%s]\n", idTag, osPathname, newVal)

						updaterFn(newVal)
						updated = true
					}
				} else if tagFrame.Encoding.Key == 0 {

					newVal := val

					okEnc := isValidEncoding(charmap.ISO8859_1, val)
					if !okEnc {
						sl, isSlavic := changeEncoding(charmap.ISO8859_1, charmap.Windows1251, val)
						if isSlavic {
							newVal = sl
						}
					}

					if newVal != val {
						fmt.Printf("file - %s\n", osPathname)
						fmt.Printf("Decoded: tag [%s] value: [%s]\n", idTag, val)
						fmt.Printf("Updated: tag [%s] value: [%s]\n", idTag, newVal)

						updaterFn(newVal)
						updated = true
					}
				}

				return updated
			}

			artistUpd := valueUpdaterFun(idTagArtist,
				artist,
				func(value string) {
					tag.SetVersion(4)
					tag.SetArtist(value)
				},
			)

			titleUpd := valueUpdaterFun(idTagTitle,
				title,
				func(value string) {
					tag.SetVersion(4)
					tag.SetTitle(value)
				},
			)

			albumUpd := valueUpdaterFun(idTagAlbum,
				album,
				func(value string) {
					tag.SetVersion(4)
					tag.SetAlbum(value)
				},
			)

			if artistUpd {
				updateCount++
			}
			if titleUpd {
				updateCount++
			}
			if albumUpd {
				updateCount++
			}

			changed := artistUpd || titleUpd || albumUpd

			if changed && !*dryRun {
				err := tag.Save()
				if err != nil {
					log.Fatal("Error while updating tag: ", err)
				}
			}

			return nil
		},
		//Unsorted: true, // (optional) set true for faster yet non-deterministic enumeration (see godoc)
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Updated %d files\n", updateCount)
}

func isValidEncoding(enc *charmap.Charmap, str string) bool {

	for _, ch := range str {

		bt, ok := enc.EncodeRune(ch)
		if !ok {
			return false
		}
		if bt < 32 || bt > 127 {
			return false
		}
	}

	return true
}

func changeEncoding(from *charmap.Charmap, to *charmap.Charmap, str string) (string, bool) {

	out := make([]rune, 0)
	for _, ch := range str {

		bt, ok := from.EncodeRune(ch)
		if !ok {
			return str, false
		}

		rn := to.DecodeByte(bt)

		out = append(out, rn)
	}
	changed := string(out)

	return changed, true
}
