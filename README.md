# deployqb

`deployqb` deploys local files as Quickbase DB pages. It supports production, development, and per-developer feature prefixes, rewrites configured page dependencies, and opens deployed index pages in a browser.

## Install

```bash
go install github.com/Justin-Stockton/deployqb@latest
```

Or build this checkout:

```bash
go build -o deployqb .
```

Go 1.22 or newer is required to build from source. The resulting binary has no runtime dependencies.

## Configure

Copy `example.env` to `.env`, fill in the values, then initialize the project:

```bash
cp example.env .env
deployqb init
```

Credentials remain in `.env`; they are not written to `qbcli.json`. Do not use real credentials on a shared or public computer.

Edit the generated `qbcli.json` and populate `filesConf`:

```json
{
  "urlQueryString": "rid=348&fid=324",
  "repositoryId": "my-project",
  "prodPrefix": "P",
  "devPrefix": "D",
  "dbid": "production-db-id",
  "devDbid": "development-db-id",
  "devAndProdQuickBaseApplications": "yes",
  "realm": "my-realm",
  "filesConf": [
    {
      "filename": "index.css",
      "path": "build/css"
    },
    {
      "filename": "index.html",
      "path": "build",
      "dependencies": [0],
      "isIndexFile": true
    }
  ]
}
```

Each dependency is an index into `filesConf`. For example, the HTML above can contain:

```html
<link href="?a=dbpage&pagename=index.css" rel="stylesheet">
```

During deployment, `deployqb` rewrites that page name with the selected environment prefix. Use `pagename`, not `pageid`, for configured dependencies.

`isIndexFile` records the returned Quickbase page ID so the corresponding launch command can open it. One index file per project is expected.

The feature prefix and feature launch page ID are per-user values stored under the operating system's user configuration directory. Change the prefix with `deployqb efeatprefix`.

## Commands

| Command | Description |
| --- | --- |
| `deployqb init` | Create or refresh `qbcli.json` from `.env`, preserving files and launch IDs |
| `deployqb feat` | Deploy with the per-user feature prefix |
| `deployqb dev` | Confirm and deploy to development |
| `deployqb prod` | Confirm and deploy to production |
| `deployqb lfeat` | Open the deployed feature index page |
| `deployqb ldev` | Open the deployed development index page |
| `deployqb lprod` | Open the deployed production index page |
| `deployqb efeatprefix` | Change the per-user feature prefix |
| `deployqb genlinks` | Print prefixed DB-page links for every configured file |
| `deployqb help` | Display command help |

## Development

```bash
go test ./...
go vet ./...
go build ./...
```

The Go implementation uses only the standard library.
