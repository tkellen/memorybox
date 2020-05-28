# memorybox ![build and test](https://github.com/tkellen/memorybox/workflows/build%20and%20test/badge.svg?branch=master) [![Coverage Status](https://coveralls.io/repos/github/tkellen/memorybox/badge.svg?branch=master&cachebust=true)](https://coveralls.io/github/tkellen/memorybox?branch=master)
> structured digital archival. simple.

# Introduction
This project aims to make the curation and replication of mixed digital media
collections simple. As a first principal, it is assumed that users will be
directly responsible for the long term storage of their data. At the same time,
it borrows ideas from distributed storage systems like [IPFS].

The design of this software is focused on autonomous operational simplicity. No
databases. No filesystem specific features. No dependency on "the cloud". No
decentralized p2p blockchain magic. Just you, a computer, a bunch of files, and
enough storage space to hold the things you've created. That's it.

## How does it work?
First, "put" some data under the management of memorybox. It will respond with
the metadata memorybox has assigned to it.
```sh
➜ printf "hello world" | memorybox put -
{"meta":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","import":{"at":"2020-05-28T17:02:47Z","source":"stdin","set":"devbox"},"memorybox":true}}

➜ memorybox put https://scaleout.team/logo.svg
{"meta":{"file":"b217de9d6cd699575ea4981761d21c9d107424d11e058cac784ad90d63e5cbe7-sha256","import":{"at":"2020-05-28T17:02:56Z","source":"https://scaleout.team/logo.svg","set":"devbox"},"memorybox":true}}

➜ memorybox put *.go
{"meta":{"file":"d0e5c438e90c4abaf8edf9d1d1278c2e099ba20e3770ca29741a173f9ebe6287-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"main.go","set":"devbox"},"memorybox":true}}
{"meta":{"file":"8b9f43e0e5df7d900ff25c8c610e7a4fe54627356c97c9f1202548899059d8ee-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"cli_test.go","set":"devbox"},"memorybox":true}}
{"meta":{"file":"34e24610f67a92e7171fbaec9e06c1b913311feba373c2694d7198619f77474b-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"cli.go","set":"devbox"},"memorybox":true}}
```

No matter where the data comes from, your imported files will end up in single
location with a flat hierarchy. By default, the destination is a folder in your
home directory called "memorybox". You can change this location, or even specify
multiple "target" stores. A new file name is computed for each file imported. No
matter how many files you import, the computed names will never conflict.
```sh
➜ find ~/memorybox -type f | sort | grep -v meta
/home/tkellen/memorybox/34e24610f67a92e7171fbaec9e06c1b913311feba373c2694d7198619f77474b-sha256
/home/tkellen/memorybox/8b9f43e0e5df7d900ff25c8c610e7a4fe54627356c97c9f1202548899059d8ee-sha256
/home/tkellen/memorybox/b217de9d6cd699575ea4981761d21c9d107424d11e058cac784ad90d63e5cbe7-sha256
/home/tkellen/memorybox/b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
/home/tkellen/memorybox/d0e5c438e90c4abaf8edf9d1d1278c2e099ba20e3770ca29741a173f9ebe6287-sha256
```
> Note: Names are deterministic. The same data will always generate the same
name.

Want to import large quantities of files and annotate them with data at the same
time? No problem. Produce a file where the first column on each line refers to a
source file on disk or retrievable via a URL. The second (optional) column is
arbitrary json data that will be included in the metadata for the file.
For example:
```
➜ cat <<EOF | memorybox import travel -
https://live.staticflickr.com/4018/5152985571_1f6631bca8_o.jpg {"group":"jpg","version":"v1","kind":"image","spec":{"id":"5152985571","name":"Nun Near Bayon in Angkor Thom"}}
https://live.staticflickr.com/4132/4997667491_794d24a3d5_o.jpg {"group":"jpg","version":"v1","kind":"image","spec":{"id":"4997667491","name":"Tyler Shoveling Away Sand"}}
https://live.staticflickr.com/4128/4997795158_733eb79733_o.jpg {"group":"jpg","version":"v1","kind":"image","spec":{"id":"4997795158","name":"Mongolian & Horse"}}
https://live.staticflickr.com/5044/5345981532_0ec5bbff9f_o.jpg {"group":"jpg","version":"v1","kind":"image","spec":{"id":"5345981532","name":"Vietnamese Pot-bellied Piggies"}}
https://live.staticflickr.com/4016/4439797619_bccc764fe8_o.jpg {"group":"jpg","version":"v1","kind":"image","spec":{"id":"4439797619","name":"Tara"}}
https://live.staticflickr.com/4034/4664252420_6172670bf6_o.jpg {"group":"jpg","version":"v1","kind":"image","spec":{"id":"4664252420","name":"Our Bikes in a German Forest"}}
EOF
queued: 6, duplicates removed: 0, existing removed: 0
{"meta":{"file":"635bac5142e7de86a2943fcbec9e57f022d82b6e298de394fde49a65b8a33eec-sha256","import":{"at":"2020-05-28T17:03:24Z","source":"https://live.staticflickr.com/4128/4997795158_733eb79733_o.jpg"},"memorybox":true},"kind":"image","spec":{"id":"4997795158","name":"Mongolian \u0026 Horse"},"group":"jpg","version":"v1"}
{"meta":{"file":"661a7dcf47c087403ca5981b58f48b7713cdf1dc49fe2036cb62fc1902e8ba9a-sha256","import":{"at":"2020-05-28T17:03:24Z","source":"https://live.staticflickr.com/4132/4997667491_794d24a3d5_o.jpg"},"memorybox":true},"group":"jpg","version":"v1","kind":"image","spec":{"id":"4997667491","name":"Tyler Shoveling Away Sand"}}
{"meta":{"file":"9ef1e9090b4a34d427c24a2a75a50148a8dc1bcef683c674704ac7cf1d771585-sha256","import":{"at":"2020-05-28T17:03:24Z","source":"https://live.staticflickr.com/4016/4439797619_bccc764fe8_o.jpg"},"memorybox":true},"spec":{"id":"4439797619","name":"Tara"},"group":"jpg","version":"v1","kind":"image"}
{"meta":{"file":"160b7f0b12cdee794db30427ecceb8429e5d8fb2c2aff7f12ccacdf1fadc357b-sha256","import":{"at":"2020-05-28T17:03:25Z","source":"https://live.staticflickr.com/4018/5152985571_1f6631bca8_o.jpg"},"memorybox":true},"spec":{"id":"5152985571","name":"Nun Near Bayon in Angkor Thom"},"group":"jpg","version":"v1","kind":"image"}
{"meta":{"file":"2bcd21b5919ef74a1a8b9d5167b8488f5f8707abbaaa81fc20b17174ddb1363e-sha256","import":{"at":"2020-05-28T17:03:25Z","source":"https://live.staticflickr.com/5044/5345981532_0ec5bbff9f_o.jpg"},"memorybox":true},"spec":{"id":"5345981532","name":"Vietnamese Pot-bellied Piggies"},"group":"jpg","version":"v1","kind":"image"}
{"meta":{"file":"43781812980cce2da36c42a002ca09a37de0c49865a339631f11a211fba059b9-sha256","import":{"at":"2020-05-28T17:03:26Z","source":"https://live.staticflickr.com/4034/4664252420_6172670bf6_o.jpg"},"memorybox":true},"kind":"image","spec":{"id":"4664252420","name":"Our Bikes in a German Forest"},"group":"jpg","version":"v1"}
```
> Note: By default, importing will keep ten transfers running at the same time.
You can tweak this to match the quantity your computer and network can support
by using the `--max=<num>` flag. Additionally, you can start and stop as often
as you like and the import functionality will always pick up where you left off.

So, how do you find your files? This tool assumes the quantity of data being
dealt with is large enough that the only meaningful way to curate it is
programatically. In order to support this, a json encoded "metafile" is created
sibling to every imported "datafile".
```sh
➜ find ~/memorybox -type f | sort
/home/tkellen/memorybox/160b7f0b12cdee794db30427ecceb8429e5d8fb2c2aff7f12ccacdf1fadc357b-sha256
/home/tkellen/memorybox/2bcd21b5919ef74a1a8b9d5167b8488f5f8707abbaaa81fc20b17174ddb1363e-sha256
/home/tkellen/memorybox/34e24610f67a92e7171fbaec9e06c1b913311feba373c2694d7198619f77474b-sha256
/home/tkellen/memorybox/43781812980cce2da36c42a002ca09a37de0c49865a339631f11a211fba059b9-sha256
/home/tkellen/memorybox/635bac5142e7de86a2943fcbec9e57f022d82b6e298de394fde49a65b8a33eec-sha256
/home/tkellen/memorybox/661a7dcf47c087403ca5981b58f48b7713cdf1dc49fe2036cb62fc1902e8ba9a-sha256
/home/tkellen/memorybox/8b9f43e0e5df7d900ff25c8c610e7a4fe54627356c97c9f1202548899059d8ee-sha256
/home/tkellen/memorybox/9ef1e9090b4a34d427c24a2a75a50148a8dc1bcef683c674704ac7cf1d771585-sha256
/home/tkellen/memorybox/b217de9d6cd699575ea4981761d21c9d107424d11e058cac784ad90d63e5cbe7-sha256
/home/tkellen/memorybox/b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
/home/tkellen/memorybox/d0e5c438e90c4abaf8edf9d1d1278c2e099ba20e3770ca29741a173f9ebe6287-sha256
/home/tkellen/memorybox/meta-160b7f0b12cdee794db30427ecceb8429e5d8fb2c2aff7f12ccacdf1fadc357b-sha256
/home/tkellen/memorybox/meta-2bcd21b5919ef74a1a8b9d5167b8488f5f8707abbaaa81fc20b17174ddb1363e-sha256
/home/tkellen/memorybox/meta-34e24610f67a92e7171fbaec9e06c1b913311feba373c2694d7198619f77474b-sha256
/home/tkellen/memorybox/meta-43781812980cce2da36c42a002ca09a37de0c49865a339631f11a211fba059b9-sha256
/home/tkellen/memorybox/meta-635bac5142e7de86a2943fcbec9e57f022d82b6e298de394fde49a65b8a33eec-sha256
/home/tkellen/memorybox/meta-661a7dcf47c087403ca5981b58f48b7713cdf1dc49fe2036cb62fc1902e8ba9a-sha256
/home/tkellen/memorybox/meta-8b9f43e0e5df7d900ff25c8c610e7a4fe54627356c97c9f1202548899059d8ee-sha256
/home/tkellen/memorybox/meta-9ef1e9090b4a34d427c24a2a75a50148a8dc1bcef683c674704ac7cf1d771585-sha256
/home/tkellen/memorybox/meta-b217de9d6cd699575ea4981761d21c9d107424d11e058cac784ad90d63e5cbe7-sha256
/home/tkellen/memorybox/meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
/home/tkellen/memorybox/meta-d0e5c438e90c4abaf8edf9d1d1278c2e099ba20e3770ca29741a173f9ebe6287-sha256
```

Initially, metafiles hold minimal data about your imported files. It is expected
that you will annotate them more fully to make them consumable by other systems
(e.g. static site generators).
```sh
➜ memorybox meta b94d27 | jq
➜ memorybox meta b94d27 | jq
{
  "meta": {
    "file": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256",
    "import": {
      "at": "2020-05-28T17:02:47Z",
      "source": "stdin",
      "set": "devbox"
    },
    "memorybox": true
  }
}
➜ memorybox meta b94d27 set demo value | jq
{
  "meta": {
    "file": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256",
    "import": {
      "at": "2020-05-28T17:02:47Z",
      "source": "stdin",
      "set": "devbox"
    },
    "memorybox": true
  },
  "demo": "value"
}
➜ memorybox meta b94d27 delete demo | jq
➜ memorybox meta b94d27 delete demo | jq
{
  "meta": {
    "file": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256",
    "import": {
      "at": "2020-05-28T17:02:47Z",
      "source": "stdin",
      "set": "devbox"
    },
    "memorybox": true
  }
}
```

All meta files can be viewed by generating an index.
```sh
➜ memorybox index
{"meta":{"file":"160b7f0b12cdee794db30427ecceb8429e5d8fb2c2aff7f12ccacdf1fadc357b-sha256","import":{"at":"2020-05-28T17:03:25Z","source":"https://live.staticflickr.com/4018/5152985571_1f6631bca8_o.jpg","set":"travel"},"memorybox":true},"spec":{"id":"5152985571","name":"Nun Near Bayon in Angkor Thom"},"group":"jpg","version":"v1","kind":"image"}
{"meta":{"file":"2bcd21b5919ef74a1a8b9d5167b8488f5f8707abbaaa81fc20b17174ddb1363e-sha256","import":{"at":"2020-05-28T17:03:25Z","source":"https://live.staticflickr.com/5044/5345981532_0ec5bbff9f_o.jpg","set":"travel"},"memorybox":true},"spec":{"id":"5345981532","name":"Vietnamese Pot-bellied Piggies"},"group":"jpg","version":"v1","kind":"image"}
{"meta":{"file":"34e24610f67a92e7171fbaec9e06c1b913311feba373c2694d7198619f77474b-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"cli.go","set":"devbox"},"memorybox":true}}
{"meta":{"file":"43781812980cce2da36c42a002ca09a37de0c49865a339631f11a211fba059b9-sha256","import":{"at":"2020-05-28T17:03:26Z","source":"https://live.staticflickr.com/4034/4664252420_6172670bf6_o.jpg","set":"travel"},"memorybox":true},"kind":"image","spec":{"id":"4664252420","name":"Our Bikes in a German Forest"},"group":"jpg","version":"v1"}
{"meta":{"file":"635bac5142e7de86a2943fcbec9e57f022d82b6e298de394fde49a65b8a33eec-sha256","import":{"at":"2020-05-28T17:03:24Z","source":"https://live.staticflickr.com/4128/4997795158_733eb79733_o.jpg","set":"travel"},"memorybox":true},"kind":"image","spec":{"id":"4997795158","name":"Mongolian \u0026 Horse"},"group":"jpg","version":"v1"}
{"meta":{"file":"661a7dcf47c087403ca5981b58f48b7713cdf1dc49fe2036cb62fc1902e8ba9a-sha256","import":{"at":"2020-05-28T17:03:24Z","source":"https://live.staticflickr.com/4132/4997667491_794d24a3d5_o.jpg","set":"travel"},"memorybox":true},"group":"jpg","version":"v1","kind":"image","spec":{"id":"4997667491","name":"Tyler Shoveling Away Sand"}}
{"meta":{"file":"8b9f43e0e5df7d900ff25c8c610e7a4fe54627356c97c9f1202548899059d8ee-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"cli_test.go","set":"devbox"},"memorybox":true}}
{"meta":{"file":"9ef1e9090b4a34d427c24a2a75a50148a8dc1bcef683c674704ac7cf1d771585-sha256","import":{"at":"2020-05-28T17:03:24Z","source":"https://live.staticflickr.com/4016/4439797619_bccc764fe8_o.jpg","set":"travel"},"memorybox":true},"spec":{"id":"4439797619","name":"Tara"},"group":"jpg","version":"v1","kind":"image"}
{"meta":{"file":"b217de9d6cd699575ea4981761d21c9d107424d11e058cac784ad90d63e5cbe7-sha256","import":{"at":"2020-05-28T17:02:56Z","source":"https://scaleout.team/logo.svg","set":"devbox"},"memorybox":true}}
{"meta":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","import":{"at":"2020-05-28T17:02:47Z","source":"stdin","set":"devbox"},"memorybox":true}}
{"meta":{"file":"d0e5c438e90c4abaf8edf9d1d1278c2e099ba20e3770ca29741a173f9ebe6287-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"main.go","set":"devbox"},"memorybox":true}}
```

Any tool that supports JSON can filter the index.
```sh
➜ memorybox index | jq -c 'select(.meta.import.source | endswith("go"))'
{"meta":{"file":"34e24610f67a92e7171fbaec9e06c1b913311feba373c2694d7198619f77474b-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"cli.go","set":"devbox"},"memorybox":true}}
{"meta":{"file":"8b9f43e0e5df7d900ff25c8c610e7a4fe54627356c97c9f1202548899059d8ee-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"cli_test.go","set":"devbox"},"memorybox":true}}
{"meta":{"file":"d0e5c438e90c4abaf8edf9d1d1278c2e099ba20e3770ca29741a173f9ebe6287-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"main.go","set":"devbox"},"memorybox":true}}
```

You can also modify any subset of the index and re-import it, updating as many
meta files as you like with a single command.
```sh
➜ memorybox index | jq -c 'select(.meta.import.source | endswith("go")) + {"demo":"key"}' | memorybox index update -
{"meta":{"file":"34e24610f67a92e7171fbaec9e06c1b913311feba373c2694d7198619f77474b-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"cli.go","set":"devbox"},"memorybox":true},"demo":"key"}
{"meta":{"file":"d0e5c438e90c4abaf8edf9d1d1278c2e099ba20e3770ca29741a173f9ebe6287-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"main.go","set":"devbox"},"memorybox":true},"demo":"key"}
{"meta":{"file":"8b9f43e0e5df7d900ff25c8c610e7a4fe54627356c97c9f1202548899059d8ee-sha256","import":{"at":"2020-05-28T17:03:03Z","source":"cli_test.go","set":"devbox"},"memorybox":true},"demo":"key"}
```

It is possible to check for missing metafiles:
```sh
➜ memorybox delete b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
➜ printf "hello world" | memorybox put -
{"meta":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","import":{"at":"2020-05-28T17:05:24Z","source":"stdin","set":"devbox"},"memorybox":true}}
➜ rm ~/memorybox/meta-b94d27*
➜ memorybox check pairing
TYPE        COUNT   SIGNATURE    SOURCE
all         21      5790dd38a3   file names
datafiles   11      eb41850139   file names
metafiles   10      0703c4cbd0   file names
unpaired    1       4544b50389   file names
b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256 missing meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
```

It is possible to check for missing datafiles:
```sh
➜ printf "hello world" | memorybox put -
{"meta":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","import":{"at":"2020-05-28T17:05:41Z","source":"stdin","set":"devbox"},"memorybox":true}}
➜ rm ~/memorybox/b94d27*
➜ memorybox check pairing
TYPE        COUNT   SIGNATURE    SOURCE
all         21      d56e48fd89   file names
datafiles   10      a12e1e2ea6   file names
metafiles   11      953f2b3009   file names
unpaired    1       14b8a7aefb   file names
meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256 missing b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
```

It is possible to check for corrupted metafiles.
```sh
➜ memorybox delete b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
➜ printf "hello world" | memorybox put -
{"meta":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","import":{"at":"2020-05-28T17:06:33Z","source":"stdin","set":"devbox"},"memorybox":true}}
➜ printf "junk[" > ~/memorybox/meta-b94d27*
➜ memorybox check metafiles
TYPE        COUNT   SIGNATURE    SOURCE
all         22      9cd7f2a71e   file names
datafiles   11      eb41850139   file names
metafiles   11      953f2b3009   file names
unpaired    0       e3b0c44298   file names
metafiles   11      0e06876b7e   file content
meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256: not json encoded
```

It is possible to check for corrupted data files:
```sh
➜ memorybox delete b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
➜ printf "hello world" | memorybox put -
{"meta":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","import":{"at":"2020-05-28T17:07:08Z","source":"stdin","set":"devbox"},"memorybox":true}}
➜ printf "junk" > ~/memorybox/b94d27*
➜ memorybox check datafiles
TYPE        COUNT   SIGNATURE    SOURCE
all         22      9cd7f2a71e   file names
datafiles   11      eb41850139   file names
metafiles   11      953f2b3009   file names
unpaired    0       e3b0c44298   file names
datafiles   11      5338bad339   file content
b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256 should be named ef875a1705a5fdac206be996f4dc1f726ea6b68861eb741c37def7277f179e37-sha256, possible data corruption
```
> Note: This can take some time as it requires reading every single bit of every
single datafile in the store (to recompute the filename hash).

There is no visual mechanism for viewing what you have stored. It is up to you
to build something to showcase it. I use this tool to support authoring a media
heavy websites that can be distributed via a USB thumb drive. More can be seen
here: https://github.com/tkellen/aevitas.

### Example Object Storage Configs
```
targets:
  aws:
    type: objectStore
    profile: profileName
    bucket: [bucket-name]
  digitalocean:
    type: objectStore
    access_key_id: ...
    secret_access_key: ...
    bucket: [spaces-name]
    endpoint: nyc3.digitaloceanspaces.com
```

## Benefits
Data can be categorized and queried using any tool that interacts with JSON.

The only system requirement for this software is free storage space.

Checking the integrity of your data is trivial.

Accidentally importing the same thing more than once is automatically detected
and prevented.

Making backup copies is very easy. Just copy the folder to another device.

## Drawbacks
Changing even a single bit in any file and trying to import it again will cause
the entire file to be duplicated. There are many highly technical approaches to
eliminating this constraint. All of them violate the primary design goal of
simplicity for this project. For maximum usability, files you manage with
memorybox should be considered "done", ideally forever. Think of it like a real,
physical memorybox.

## Try it
Clone this repo and run the following:

```sh
go build && ./memorybox
```

### Prior Art (in order of my becoming aware of them)
* [Scuttlebutt](https://scuttlebutt.nz/)
* [IPFS]
* [Dat](https://dat.foundation/)
* [Filecoin](https://filecoin.io/)
* [Perkeep](https://perkeep.org/)
* [casync](https://github.com/systemd/casync)
* [Storj](https://storj.io/)
* [SAFE Network](https://safenetwork.tech/)

[jsonl]: http://jsonlines.org/
[IPFS]: https://ipfs.io/