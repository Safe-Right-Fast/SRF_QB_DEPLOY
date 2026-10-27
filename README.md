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

Go 1.22 or newer is required to build from source.

## Configure

Run `deployqb init` from each project root. The interactive setup asks for project values and lets you create or reuse named account profiles. Existing profile names are shown during setup; enter one to reuse its realm and credentials.

Project settings and `filesConf` are stored in `qbcli.json`. Named profiles are stored in the operating system user configuration directory in `configstore/deployqb.json` with permissions restricted to the current user. Profiles contain a Quickbase realm, user token, and optional app token. Credentials are not written to `qbcli.json`.

The process environment may still supply legacy credentials (`QB_USERTOKEN`, `QB_APP_TOKEN`, `QB_DEV_USERTOKEN`, and related names), but `.env` files are not loaded automatically.

After initialization, edit the generated `qbcli.json` and populate `filesConf`:

```json
{
  "urlQueryString": "rid=348&fid=324",
  "repositoryId": "my-project",
  "prodProfile": "acme-prod",
  "devProfile": "acme-dev",
  "prodPrefix": "P",
  "devPrefix": "D",
  "dbid": "production-db-id",
  "devDbid": "development-db-id",
  "devAndProdQuickBaseApplications": "yes",
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

For Vite, Webpack, Rollup, or another bundler that generates hashed asset names, set `source` to a glob while keeping `filename` stable:

```json
"filesConf": [
  { "filename": "app.js", "path": "dist/assets", "source": "*.js" },
  { "filename": "app.css", "path": "dist/assets", "source": "*.css" },
  {
    "filename": "index.html",
    "path": "dist",
    "dependencies": [0, 1],
    "isIndexFile": true
  }
]
```

Run the project's build before `deployqb`. Each `source` must match exactly one file. The generated HTML asset paths are replaced with prefixed Quickbase page links containing a content-hash cache key. This intentionally supports a single JavaScript and CSS bundle; add manifest-based entries if the application starts code-splitting.

`isIndexFile` records the returned Quickbase page ID so the corresponding launch command can open it. One index file per project is expected.

The feature prefix and feature launch page ID are per-user values stored under the operating system's user configuration directory. Change the prefix with `deployqb efeatprefix`.

## Commands

| Command | Description |
| --- | --- |
| `deployqb init` | Interactively create or refresh `qbcli.json` and account profiles |
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
