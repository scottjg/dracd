#ifndef GOGLUE_H
#define GOGLUE_H

#include <stdlib.h>
#include "glue.h"

typedef struct go_client_ctx
{
	client_ctx lib;
	void *client_data;
} go_client_ctx;

client_ctx *alloc_client_ctx(void *client_data, int drac_type);
void free_client_ctx(client_ctx *ctx);
void *get_client_data(client_ctx *ctx);
int get_width(client_ctx *ctx);
int get_height(client_ctx *ctx);
void *get_fb(client_ctx *ctx);

size_t encode_png(void *frame, unsigned short width, unsigned short height, void **out);

#endif