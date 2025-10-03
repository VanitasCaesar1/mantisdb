#include "storage_engine.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// External functions from btree.c and buffer_pool.c
extern btree_t* btree_create(const char* filename);
extern void btree_destroy(btree_t* tree);
extern int btree_insert(btree_t* tree, const char* key, const char* value);
extern char* btree_search(btree_t* tree, const char* key);
extern int btree_delete(btree_t* tree, const char* key);

extern buffer_pool_t* buffer_pool_create(size_t pool_size);
extern void buffer_pool_destroy(buffer_pool_t* pool);

storage_engine_t* storage_engine_init(const char* data_dir) {
    storage_engine_t* engine = malloc(sizeof(storage_engine_t));
    if (!engine) return NULL;
    
    engine->data_dir = strdup(data_dir);
    engine->buffer_pool = buffer_pool_create(1024 * 1024); // 1MB buffer pool
    
    char btree_path[512];
    snprintf(btree_path, sizeof(btree_path), "%s/data.btree", data_dir);
    engine->btree = btree_create(btree_path);
    
    if (!engine->btree || !engine->buffer_pool) {
        storage_engine_cleanup(engine);
        return NULL;
    }
    
    return engine;
}

void storage_engine_cleanup(storage_engine_t* engine) {
    if (!engine) return;
    
    if (engine->btree) btree_destroy(engine->btree);
    if (engine->buffer_pool) buffer_pool_destroy(engine->buffer_pool);
    if (engine->data_dir) free(engine->data_dir);
    free(engine);
}

int storage_engine_put(storage_engine_t* engine, const char* key, const char* value) {
    if (!engine || !key || !value) return -1;
    return btree_insert(engine->btree, key, value);
}

char* storage_engine_get(storage_engine_t* engine, const char* key) {
    if (!engine || !key) return NULL;
    return btree_search(engine->btree, key);
}

int storage_engine_delete(storage_engine_t* engine, const char* key) {
    if (!engine || !key) return -1;
    return btree_delete(engine->btree, key);
}