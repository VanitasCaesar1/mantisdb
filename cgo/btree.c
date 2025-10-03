#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#define MAX_KEYS 255
#define MAX_CHILDREN 256

typedef struct btree_node {
    char keys[MAX_KEYS][256];
    char values[MAX_KEYS][1024];
    struct btree_node* children[MAX_CHILDREN];
    int num_keys;
    bool is_leaf;
} btree_node_t;

typedef struct btree {
    btree_node_t* root;
    char* filename;
} btree_t;

btree_node_t* btree_node_create(bool is_leaf) {
    btree_node_t* node = calloc(1, sizeof(btree_node_t));
    if (node) {
        node->is_leaf = is_leaf;
        node->num_keys = 0;
    }
    return node;
}

btree_t* btree_create(const char* filename) {
    btree_t* tree = malloc(sizeof(btree_t));
    if (!tree) return NULL;
    
    tree->filename = strdup(filename);
    tree->root = btree_node_create(true);
    
    if (!tree->root) {
        free(tree->filename);
        free(tree);
        return NULL;
    }
    
    return tree;
}

void btree_node_destroy(btree_node_t* node) {
    if (!node) return;
    
    if (!node->is_leaf) {
        for (int i = 0; i <= node->num_keys; i++) {
            btree_node_destroy(node->children[i]);
        }
    }
    free(node);
}

void btree_destroy(btree_t* tree) {
    if (!tree) return;
    
    btree_node_destroy(tree->root);
    free(tree->filename);
    free(tree);
}

int btree_insert(btree_t* tree, const char* key, const char* value) {
    if (!tree || !key || !value) return -1;
    
    // Simple implementation - just add to root if space available
    btree_node_t* root = tree->root;
    if (root->num_keys < MAX_KEYS) {
        strcpy(root->keys[root->num_keys], key);
        strcpy(root->values[root->num_keys], value);
        root->num_keys++;
        return 0;
    }
    
    return -1; // Tree full (simplified implementation)
}

char* btree_search(btree_t* tree, const char* key) {
    if (!tree || !key) return NULL;
    
    btree_node_t* root = tree->root;
    for (int i = 0; i < root->num_keys; i++) {
        if (strcmp(root->keys[i], key) == 0) {
            return strdup(root->values[i]);
        }
    }
    
    return NULL;
}

int btree_delete(btree_t* tree, const char* key) {
    if (!tree || !key) return -1;
    
    btree_node_t* root = tree->root;
    for (int i = 0; i < root->num_keys; i++) {
        if (strcmp(root->keys[i], key) == 0) {
            // Shift remaining keys/values
            for (int j = i; j < root->num_keys - 1; j++) {
                strcpy(root->keys[j], root->keys[j + 1]);
                strcpy(root->values[j], root->values[j + 1]);
            }
            root->num_keys--;
            return 0;
        }
    }
    
    return -1; // Key not found
}