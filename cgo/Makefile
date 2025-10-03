CC = gcc
CFLAGS = -Wall -Wextra -std=c99 -fPIC -O2
LDFLAGS = -shared

SOURCES = storage_engine.c btree.c buffer_pool.c
OBJECTS = $(SOURCES:.c=.o)
TARGET = libstorage_engine.so

.PHONY: all clean

all: $(TARGET)

$(TARGET): $(OBJECTS)
	$(CC) $(LDFLAGS) -o $@ $^

%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@

clean:
	rm -f $(OBJECTS) $(TARGET)

install: $(TARGET)
	cp $(TARGET) ../storage/

.PHONY: install