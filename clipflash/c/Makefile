# Makefile for main.c

TARGET = clipflash.exe
SRC = main.c
CC = gcc
CFLAGS = -municode -mwindows
LIBS = -lole32 -lshell32

all: $(TARGET)

$(TARGET): $(SRC)
	$(CC) $(SRC) -o $(TARGET) $(CFLAGS) $(LIBS)

clean:
	del /Q $(TARGET) 2>nul || true
