#ifndef STORAGE_ENGINE_H
#define STORAGE_ENGINE_H

#include <stdint.h>
#include <stdbool.h>

// Forward declarations
typedef struct btree btree_t;
typedef struct buffer_pool buffer_pool_t;

// Storage engine context
typedef struct {
    btree_t *btree;
    buffer_pool_t *buffer_pool;
    char *data_dir;
} storage_engine_t;

// Initialize storage engine
storage_engine_t* storage_engine_init(const char* data_dir);

// Cleanup storage engine
void storage_engine_cleanup(storage_engine_t* engine);

// Basic operations
int storage_engine_put(storage_engine_t* engine, const char* key, const char* value);
char* storage_engine_get(storage_engine_t* engine, const char* key);
int storage_engine_delete(storage_engine_t* engine, const char* key);

#endif // STORAGE_ENGINE_H