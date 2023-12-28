# Bcrypter

## Install

```sh
$ go install github.com/w00j00ng/goproj/bcrypter@latest
```

## Run

1.1. Enter Password

```sh
$ bcrypter
Enter Password: 
```

1.2. Check bcrypted text

```sh
$ bcrypter
$2a$10$V3nzSqajt.5FDZ/b/wW88eBpeMMYUr8f8swjjVw7pMOghGTrRlxQG
```

2.1. If you don't want to print the bcrypted text on the console then you should run it like below

```sh
> bcrypter --clipboard=true
Enter Password: 
```

2.2. Check Password

```sh
$ bcrypter --clipboard=true
copied on the clipboard!
```
