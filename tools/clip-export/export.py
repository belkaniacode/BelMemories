#!/usr/bin/env python3
"""Export the CLIP image encoder to ONNX and precompute label text embeddings.

Run once by a developer; the output goes to ../../models:
    models/clip-image.onnx   – image encoder (int8 dynamic quantisation)
    models/clip-labels.json  – normalised text embeddings of category prompts

Usage:
    python -m venv .venv && . .venv/bin/activate
    pip install -r requirements.txt
    python export.py [--out ../../models] [--no-quantize] [--labels-only]
"""

import argparse
import inspect
import json
import os
import sys

import torch
from transformers import (
    CLIPTokenizer,
    CLIPTextModelWithProjection,
    CLIPVisionModelWithProjection,
)

MODEL_ID = "openai/clip-vit-base-patch32"

# Each label averages several prompts. "group" decides the archive folder:
# photo → Фото, picture → Картинки. Labels in HUMAN mark images with people:
# the app keeps those in Фото unless the picture score is overwhelming.
# Prompts were calibrated on real phone/social-network photos vs screenshots
# and app icons (people in cars, on vehicles, low-quality photos used to
# leak into Картинки).
LABELS = [
    ("person", "photo", ["a photo of a person", "a portrait photo of a man or a woman", "a selfie",
                         "an amateur photo of a young man", "a photo of a man in sunglasses"]),
    ("people", "photo", ["a group photo of people", "a photo of a family", "a photo of friends at a party",
                         "a photo of friends hugging"]),
    ("child", "photo", ["a photo of a child", "a photo of a baby"]),
    ("driver", "photo", ["a photo of a man driving a car", "a photo of a person sitting in a car",
                         "a photo of a person riding a motorbike or a quad bike"]),
    ("animal", "photo", ["a photo of a pet", "a photo of a dog", "a photo of a cat", "a photo of an animal"]),
    ("nature", "photo", ["a landscape photo", "a photo of nature", "a photo of the sea", "a photo of mountains"]),
    ("city", "photo", ["a photo of a city street", "a photo of a building", "a travel photo"]),
    ("food", "photo", ["a photo of food", "a photo of a meal on a table"]),
    ("indoor", "photo", ["a photo of a room", "a photo taken indoors at home", "a photo of a car"]),
    ("event", "photo", ["a photo of a wedding", "a photo of a birthday celebration", "a photo of a concert"]),
    ("oldphoto", "photo", ["an old low quality photo", "a blurry phone photo", "a scanned old photograph"]),
    ("logo", "picture", ["a company logo", "a brand logo on a white background", "an emblem"]),
    ("icon", "picture", ["an app icon", "a flat icon", "a pictogram"]),
    ("screenshot", "picture", ["a screenshot of a phone screen", "a screenshot of a computer screen",
                               "a screenshot of a chat", "a screenshot of a website"]),
    ("document", "picture", ["a scanned document with text", "a photo of a receipt", "a page of text",
                             "a table or a chart"]),
    ("meme", "picture", ["a meme with text", "a greeting card with text", "a postcard with an inscription"]),
    ("illustration", "picture", ["a digital illustration", "a cartoon drawing", "clip art", "a vector graphic",
                                 "an anime drawing", "a 3d render"]),
    ("banner", "picture", ["an advertising banner", "a poster with text", "a sticker"]),
]
HUMAN = {"person", "people", "child", "driver"}


class ImageEncoder(torch.nn.Module):
    def __init__(self, model):
        super().__init__()
        self.model = model

    def forward(self, pixel_values):
        return self.model(pixel_values=pixel_values).image_embeds


def export_image(out_dir: str, quantize: bool) -> None:
    vision = CLIPVisionModelWithProjection.from_pretrained(MODEL_ID, use_safetensors=True).eval()
    wrapper = ImageEncoder(vision)
    dummy = torch.randn(1, 3, 224, 224)
    fp32_path = os.path.join(out_dir, "clip-image.fp32.onnx")
    final_path = os.path.join(out_dir, "clip-image.onnx")
    kwargs = {}
    if "dynamo" in inspect.signature(torch.onnx.export).parameters:
        kwargs["dynamo"] = False  # newer torch defaults to the dynamo exporter
    torch.onnx.export(
        wrapper,
        (dummy,),
        fp32_path,
        input_names=["pixel_values"],
        output_names=["image_embeds"],
        dynamic_axes={"pixel_values": {0: "batch"}, "image_embeds": {0: "batch"}},
        opset_version=17,
        **kwargs,
    )
    if quantize:
        from onnxruntime.quantization import QuantType, quantize_dynamic

        quantize_dynamic(fp32_path, final_path, weight_type=QuantType.QUInt8)
        os.remove(fp32_path)
    else:
        os.replace(fp32_path, final_path)
    print(f"image encoder -> {final_path} ({os.path.getsize(final_path) / 1e6:.1f} MB)")


@torch.no_grad()
def export_labels(out_dir: str) -> None:
    tokenizer = CLIPTokenizer.from_pretrained(MODEL_ID)
    text = CLIPTextModelWithProjection.from_pretrained(MODEL_ID, use_safetensors=True).eval()
    labels = []
    for name, group, prompts in LABELS:
        tokens = tokenizer(prompts, padding=True, return_tensors="pt")
        emb = text(**tokens).text_embeds
        emb = emb / emb.norm(dim=-1, keepdim=True)
        mean = emb.mean(dim=0)
        if not torch.isfinite(mean).all() or mean.norm() == 0:
            raise RuntimeError(f"broken text embedding for {name!r}: re-download the model weights")
        mean = mean / mean.norm()
        labels.append({"label": name, "group": group, "human": name in HUMAN, "prompts": prompts,
                       "embedding": mean.tolist()})
    dim = len(labels[0]["embedding"])
    path = os.path.join(out_dir, "clip-labels.json")
    with open(path, "w", encoding="utf-8") as f:
        json.dump({"model": MODEL_ID, "dim": dim, "logitScale": 100.0, "labels": labels}, f)
    print(f"labels ({len(labels)}) -> {path}")


def main() -> int:
    here = os.path.dirname(os.path.abspath(__file__))
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", default=os.path.join(here, "..", "..", "models"))
    parser.add_argument("--no-quantize", action="store_true")
    parser.add_argument("--labels-only", action="store_true", help="only regenerate clip-labels.json")
    args = parser.parse_args()
    os.makedirs(args.out, exist_ok=True)
    if not args.labels_only:
        export_image(args.out, not args.no_quantize)
    export_labels(args.out)
    return 0


if __name__ == "__main__":
    sys.exit(main())
