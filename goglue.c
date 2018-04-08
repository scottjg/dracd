#include <stdio.h>
#include <string.h>
#include <png.h>

#include "goglue.h"
#include "decoder.h"

extern int GlueReadDataCtrl(client_ctx *ctx, size_t size);
extern int GlueWriteDataCtrl(client_ctx *ctx, void *data, size_t size);
extern int GlueStartSslCtrl(client_ctx *ctx);
extern int GlueStartSslVideo(client_ctx *ctx);
extern int GlueReadDataVideo(client_ctx *ctx, size_t size);
extern int GlueWriteDataVideo(client_ctx *ctx, void *data, size_t size);

client_ctx *alloc_client_ctx(void *client_data, int drac_type)
{
    go_client_ctx *ctx = calloc(sizeof(go_client_ctx), 1);
    ctx->lib.glue_write_data_ctrl = GlueWriteDataCtrl;
    ctx->lib.glue_write_data_video = GlueWriteDataVideo;
    ctx->lib.glue_read_data_ctrl = GlueReadDataCtrl;
    ctx->lib.glue_read_data_video = GlueReadDataVideo;
    ctx->lib.glue_start_ssl_ctrl = GlueStartSslCtrl;
    ctx->lib.glue_start_ssl_video = GlueStartSslVideo;

    ctx->client_data = client_data;
    //printf("client_data=%p\n", client_data);
    init_decoder((client_ctx *)ctx);

    ctx->lib.dracType = drac_type;

    return &ctx->lib;
}

void free_client_ctx(client_ctx *ctx)
{
    go_client_ctx *c = (go_client_ctx *)ctx;
    free(c->lib.user);
    free(c->lib.passwd);
    free(c->lib.packet_buffer);
    free(c->lib.framebuffer);
	free(c);
}

void *get_client_data(client_ctx *ctx)
{
    go_client_ctx *c = (go_client_ctx *)ctx;
    return c->client_data;
}

int get_width(client_ctx *ctx)
{
    return ctx->width;
}

int get_height(client_ctx *ctx)
{
    return ctx->height;
}

void *get_fb(client_ctx *ctx)
{
    return ctx->framebuffer;
}


typedef struct png_ctx {
    void *out;
    size_t out_len;
    size_t out_capacity;    
} png_ctx;

void my_png_write_data(png_structp png_ptr, png_bytep data, png_size_t length)
{
    png_ctx *ctx = (png_ctx *)png_get_io_ptr(png_ptr);
    if (ctx->out_len + length > ctx->out_capacity) {
        size_t new_size = ctx->out_capacity > 0 ? ctx->out_capacity * 2 : 4096;
        if (ctx->out_len + length > new_size) {
            new_size = ctx->out_len + length;
        }
        ctx->out = realloc(ctx->out, new_size);
        ctx->out_capacity = new_size;
    }
    memcpy(&((uint8_t *)ctx->out)[ctx->out_len], data, length);
    ctx->out_len += length;
}

size_t encode_png(void *image, unsigned short width, unsigned short height, void **out)
{
    png_structp png;
    png_infop info;
    png_ctx ctx = { 0 };

    unsigned char *frame = (unsigned char *)image;
    png = png_create_write_struct(PNG_LIBPNG_VER_STRING, NULL, NULL, NULL);
    info = png_create_info_struct(png);
    if(setjmp(png_jmpbuf(png))) {
        fprintf(stderr, "png error!\n");
        exit(1);
    }

    png_set_IHDR(png,
                 info,
                 width,
                 height,
                 8,
                 PNG_COLOR_TYPE_RGB_ALPHA,
                 PNG_INTERLACE_NONE,
                 PNG_COMPRESSION_TYPE_DEFAULT,
                 PNG_FILTER_TYPE_DEFAULT);
    png_set_bgr(png);
    png_set_filter(png, 0, PNG_FILTER_NONE);
    png_set_write_fn(png, &ctx, my_png_write_data, NULL);
    png_write_info(png, info);
    //printf("writing %d pixels\n", width * height * 4);
    size_t pixels = width * height * 4;
    for (unsigned i = 0; i < pixels; i += width * 4) {
        png_write_row(png, &frame[i]);
    }
    png_write_end(png, NULL);
    png_destroy_write_struct(&png, &info);
    //printf("png size=%zu\n", ctx.out_len);

    *out = ctx.out;
    return ctx.out_len;
}
