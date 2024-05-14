# fixID3tags

Tool to fix incorrect tag encoding in mp3 files.

build:
```sh
go build
```

usage:
```sh
$ ./fixID3tags
tag fix
  -dry-run
    	dry run, no changes (default true)
  -fix-ISO8859-1
    	undo UTF8 tag read as ISO8859-1
  -fix-title-only
    	parse and fix title only
  -music-dir string
    	path to music folder
  -override-album string
    	set new value for album tag
  -override-artist string
    	set new value for artist tag
  -skip-empty-tags
    	skip empty tags
```

Some mp3 files have tags encoded as ISO8859-1 but have russian text inside (cp1251). To fix this and encode as utf8 do
```sh
./fixID3tags -music-dir="<MUSIC>"
``` 
this will run in dry mode and show suggested changes. To apply changes, run
```sh
./fixID3tags -music-dir="<MUSIC>" -dry-run=false
``` 

Some mp3 files have incorrectly set encoding as ISO8859-1 but actual data is in UTF8, to fix this do
```sh
./fixID3tags -music-dir="<MUSIC>" -fix-ISO8859-1=true
``` 
