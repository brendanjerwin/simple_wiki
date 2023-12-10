
*simple_wiki* is just that: a simple wiki.

Getting Started
===============

## Install

If you have go

```
go get -u github.com/brendanjerwin/simple_wiki/...
```

## Run

To run just double click or from the command line:

```
simple_wiki
```

and it will start a server listening  on `0.0.0.0:8050`. To view it, just go to http://localhost:8050 (the server prints out the local IP for your info if you want to do LAN networking). You can change the port with `-port X`, and you can listen *only* on localhost using `-host localhost`.

## Server customization

There are a couple of command-line flags that you can use to make *simple_wiki* your own micro-CMS. 

```
simple_wiki -lock 123 -default-page index.html -css mystyle.css
```

The `-lock` flag will automatically lock every page with the passphrase "123". Also, the default behavior will be to redirect `/` to `/index.html`. 

## Usage

*simple_wiki* is straightforward to use. Here are some of the basic features:

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

To the original project I started from: https://github.com/schollz/cowyo 

## License

MIT

