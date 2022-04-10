package main

import (
	"flag"
	"fmt"
	"log"
	"os"
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
	musicDirPtr := flag.String("music-dir", "", "path to music folder")
	overrideArtistPtr := flag.String("override-artist", "", "set new value for artist tag")
	overrideAlbumPtr := flag.String("override-album", "", "set new value for album tag")
	skipEmptyTags := flag.Bool("skip-empty-tags", false, "skip empty tags")
	fixReadUtf8AsISO8859 := flag.Bool("fix-ISO8859-1", false, "undo UTF8 tag read as ISO8859-1")
	minimalTagParse := flag.Bool("fix-title-only", false, "parse and fix title only")

	flag.Parse()

	changeActions := map[string]ChangeStrategy{
		idTagArtist: {
			ChangeAction_Owerride,
			*overrideArtistPtr,
		},
		idTagAlbum: {
			ChangeAction_Owerride,
			*overrideAlbumPtr,
		},
	}

	if len(*overrideAlbumPtr) == 0 {
		changeActions[idTagAlbum] = ChangeStrategy{
			changeAction: ChangeAction_Halt,
		}
	}

	changeActions[idTagTitle] = ChangeStrategy{
		changeAction: ChangeAction_Skip,
	}

	if *skipEmptyTags {
		changeActions[idTagAlbum] = ChangeStrategy{
			changeAction: ChangeAction_Skip,
		}
		changeActions[idTagArtist] = ChangeStrategy{
			changeAction: ChangeAction_Skip,
		}
	}

	dirname := *musicDirPtr
	if len(dirname) == 0 {
		flag.PrintDefaults()
		return
	}

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

			parseFields := []string{idTagArtist, idTagAlbum, idTagTitle}
			if *minimalTagParse {
				parseFields = []string{idTagTitle}
			}

			tag, err := id3v2.Open(osPathname,
				id3v2.Options{
					Parse:       true,
					ParseFrames: parseFields,
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
						if len(newVal) > 0 {
							fmt.Printf("EMPTY [%s] (file: %s), set to [%s]\n", idTag, osPathname, newVal)

							updaterFn(newVal)
							updated = true
						}
					} else if changeActions[idTag].changeAction == ChangeAction_Halt {
						fmt.Printf("EMPTY [%s] (file: %s), halt\n", idTag, osPathname)
						os.Exit(0)
					}
				} else if tagFrame.Encoding.Key == 0 {

					newVal := val

					if *fixReadUtf8AsISO8859 {
						out := make([]byte, 0)
						for _, rn := range val {

							r0, _ := charmap.ISO8859_1.EncodeRune(rn)

							out = append(out, r0)
						}
						newVal = string(out)
					} else {
						okEnc := isValidEncoding(charmap.ISO8859_1, val)
						if !okEnc {
							sl, isSlavic := changeEncoding(charmap.ISO8859_1, charmap.Windows1251, val)
							if isSlavic {
								newVal = sl
							}
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
