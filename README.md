# go-bdinfo

Go rewrite of BDInfo.

## Usage

```sh
go run ./cmd/bdinfo -p /path/to/bluray
```

Report default: `BDInfo_{0}.bdinfo` (disc label substituted).

## Options

- `-p, --path` (required)
- `-o, --reportfilename`
- `--main` (only main playlist; likely what you want)
- `-f, --forumsonly` (only forums paste block)
- `-s, --summaryonly` (only quick summary block; likely what you want)
- `-b, --enablessif`
- `-l, --filterloopingplaylists`
- `-y, --filtershortplaylist`
- `-v, --filtershortplaylistvalue`
- `-k, --keepstreamorder`
- `-m, --generatetextsummary`
- `-q, --includeversionandnotes`
- `-j, --groupbytime`
- `-g, --generatestreamdiagnostics`
- `-e, --extendedstreamdiagnostics`
