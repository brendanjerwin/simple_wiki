# simple_wiki is just that: a simple wiki

## Getting Started

## Install

If you have Go:

```go
go install github.com/brendanjerwin/simple_wiki@latest
```

## Run

To run from the command line:

```shell
simple_wiki
```

and it will start a server listening on `0.0.0.0:8050`. To view it, just go to <http://localhost:8050> (the server prints out the local IP for your info if you want to do LAN networking). You can change the port with `-port X`, and you can listen _only_ on localhost using `-host localhost`.

## Server customization

There are a couple of command-line flags that you can use to make _simple_wiki_ your own micro-CMS.

```shell
simple_wiki -lock 123 -default-page index.html -css mystyle.css
```

The `-lock` flag will automatically lock every page with the passphrase "123". Also, the default behavior will be to redirect `/` to `/index.html`.

## Usage

_simple_wiki_ is straightforward to use. Here are some of the basic features:

### View all the pages

To view the current list of all the pages goto to `/ls`.

### Editing

When you open a document you'll be directed to an alliterative animal (which is supposed to be easy to remember). You can write in Markdown. Saving is performed as soon as you stop writing. You can easily link pages using [[PageName]] as you edit.

![Editing](http://i.imgur.com/vEs2U8z.gif)

### History

You can easily see previous versions of your documents.

![History](http://i.imgur.com/CxhRkyo.gif)

### Locking

Locking prevents other users from editing your pages without a passphrase.

![Locking](http://i.imgur.com/xwUFV8b.gif)

## Thanks

To the original project I started from: [cowyo](https://github.com/schollz/cowyo).

## License

MIT
