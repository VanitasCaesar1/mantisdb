#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

#define PAGE_SIZE 4096

typedef struct buffer_page {
    char data[PAGE_SIZE];
    int page_id;
    bool dirty;
    bool pinned;
    int ref_count;
} buffer_page_t;

typedef struct buffer_pool {
    buffer_page_t* pages;
    size_t pool_size;
    size_t num_pages;
    int* free_list;
    size_t free_count;
} buffer_pool_t;

buffer_pool_t* buffer_pool_create(size_t pool_size) {
    buffer_pool_t* pool = malloc(sizeof(buffer_pool_t));
    if (!pool) return NULL;
    
    pool->pool_size = pool_size;
    pool->num_pages = pool_size / PAGE_SIZE;
    
    pool->pages = calloc(pool->num_pages, sizeof(buffer_page_t));
    pool->free_list = malloc(pool->num_pages * sizeof(int));
    
    if (!pool->pages || !pool->free_list) {
        free(pool->pages);
        free(pool->free_list);
        free(pool);
        return NULL;
    }
    
    // Initialize free list
    for (size_t i = 0; i < pool->num_pages; i++) {
        pool->free_list[i] = i;
    }
    pool->free_count = pool->num_pages;
    
    return pool;
}

void buffer_pool_destroy(buffer_pool_t* pool) {
    if (!pool) return;
    
    free(pool->pages);
    free(pool->free_list);
    free(pool);
}

buffer_page_t* buffer_pool_get_page(buffer_pool_t* pool, int page_id) {
    if (!pool || pool->free_count == 0) return NULL;
    
    // Simple implementation - just return first free page
    int free_idx = pool->free_list[--pool->free_count];
    buffer_page_t* page = &pool->pages[free_idx];
    
    page->page_id = page_id;
    page->dirty = false;
    page->pinned = true;
    page->ref_count = 1;
    
    return page;
}

void buffer_pool_release_page(buffer_pool_t* pool, buffer_page_t* page) {
    if (!pool || !page) return;
    
    page->ref_count--;
    if (page->ref_count == 0) {
        page->pinned = false;
        // Add back to free list
        pool->free_list[pool->free_count++] = page - pool->pages;
    }
}