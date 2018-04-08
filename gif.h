#include <gif_lib.h>

typedef struct QuantizedFrame {
	void *buffer;
	ColorMapObject *colormap;
	int width, height;
	int delay;
} QuantizedFrame;

void QuantizeFrame(void *inputFrame, int width, int height, QuantizedFrame *qf);
int WriteGif(QuantizedFrame *qframes, int frameCount, void **frameOut);
void FreeQuantizedFrame(QuantizedFrame *qf);