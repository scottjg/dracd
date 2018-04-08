#ifndef DISABLE_GIF

#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>

#include "gif.h"

#if !(defined(GIFLIB_MAJOR) && GIFLIB_MAJOR >= 5)
#define GifMakeMapObject MakeMapObject
#define GifFreeMapObject FreeMapObject
#define GifQuantizeBuffer QuantizeBuffer
#endif

//XXX need error handling

void QuantizeFrame(void *inputFrame, int width, int height, QuantizedFrame *qf) {
	int colormap_size = 256;
	uint8_t *rgba = (uint8_t *)inputFrame;

	GifByteType *red = malloc(width * height * sizeof(GifByteType));
	GifByteType *green = malloc(width * height * sizeof(GifByteType));
	GifByteType *blue = malloc(width * height * sizeof(GifByteType));

	qf->buffer = malloc(width * height * sizeof(GifByteType));
	qf->colormap = GifMakeMapObject(colormap_size, NULL);

	for (int i = 0; i < width * height; i++) {
		red[i]   = (GifByteType) rgba[4 * i + 2];
		green[i] = (GifByteType) rgba[4 * i + 1];
		blue[i]  = (GifByteType) rgba[4 * i    ];
	}

  	GifQuantizeBuffer(width, height, &colormap_size, red, green, blue,   
	                  qf->buffer, qf->colormap->Colors);
	free(red);
	free(green);
	free(blue);
	qf->width = width;
	qf->height = height;
}

typedef struct WriteBuffer {
	char *buffer;
	int offset;
	int size;
} WriteBuffer;

int WriteGifWriteFunc(GifFileType *gif, const GifByteType *buffer, int size) {
	int new_size;
	WriteBuffer *wb = (WriteBuffer *)gif->UserData;
	int desired_size = wb->offset + size;
	if (desired_size > wb->size) {
		if (wb->size == 0)
			new_size = 65536;
		else
			new_size = wb->size * 2;
		if (new_size < desired_size)
			new_size = desired_size;
		wb->buffer = realloc(wb->buffer, new_size);
		wb->size = new_size;
	}

	memcpy(&wb->buffer[wb->offset], buffer, size);
	wb->offset += size;
	return size;
}


//XXX needs to handle the case where the size of the frame changes
int WriteGif(QuantizedFrame *qframes, int frameCount, void **frameOut) {
	int err;
	WriteBuffer wb = { 0 };
	GifFileType *ft = 
#if defined(GIFLIB_MAJOR) && GIFLIB_MAJOR >= 5
		EGifOpen(&wb, WriteGifWriteFunc, &err);
#else
		EGifOpen(&wb, WriteGifWriteFunc);
#endif

#if defined(GIFLIB_MAJOR) && GIFLIB_MAJOR >= 5
	EGifSetGifVersion(ft, 1);
#else
	EGifSetGifVersion("89a");
#endif

	EGifPutScreenDesc(ft, qframes[0].width, qframes[0].height,
		256 /*colormap_size*/, 0, qframes[0].colormap);

	// gif loop header
#if defined(GIFLIB_MAJOR) && GIFLIB_MAJOR >= 5
	EGifPutExtensionLeader(ft, APPLICATION_EXT_FUNC_CODE);
	EGifPutExtensionBlock(ft, 11, "NETSCAPE2.0");
	EGifPutExtensionBlock(ft, 3, "\x01\x00\x00");
	EGifPutExtensionTrailer(ft);
#else
	EGifPutExtensionFirst(ft, APPLICATION_EXT_FUNC_CODE, 11, "NETSCAPE2.0");
	EGifPutExtensionLast(ft, APPLICATION_EXT_FUNC_CODE, 3, "\x01\x00\x00");
#endif

	for (int i = 0; i < frameCount; i ++) {
		uint8_t ctrlblock[4] = { 
			qframes[i].delay >> 8,
			qframes[i].delay & 0xff,
			0, 0 };
		EGifPutExtension(ft, GRAPHICS_EXT_FUNC_CODE, 4, ctrlblock);		
		EGifPutImageDesc(ft, 0, 0, qframes[i].width, qframes[i].height,
			0, qframes[i].colormap);

		GifPixelType *ptr = (GifPixelType *)qframes[i].buffer;
		for (int j = 0; j < qframes[i].height; j++) {
			EGifPutLine(ft, ptr, qframes[i].width);
			ptr += qframes[i].width;
		}
	}

#if defined(GIFLIB_MAJOR) && GIFLIB_MAJOR >= 5 && GIFLIB_MINOR >= 1
	EGifCloseFile(ft, &err);
#else
	EGifCloseFile(ft);
#endif

	*frameOut = wb.buffer;
	return wb.offset;
}

void FreeQuantizedFrame(QuantizedFrame *qf) {
	if (qf->colormap)
		GifFreeMapObject(qf->colormap);
	if (qf->buffer)
		free(qf->buffer);
}

#endif