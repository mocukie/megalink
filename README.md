# MEGAlink

Download mega public file with your favorite downloader (e.g. aria2, idm).

## Usage

```bash
megalink --addr 127.0.0.1:30303
```

Open http://127.0.0.1:30303, and then input your link.

Valid link format:

```
https://mega.nz/${node}!${key}
https://mega.nz/file/${node}#${key}
https://mega.nz/folder/${node}#${key}/file/${node}
```

## License

[MIT](LICENSE)