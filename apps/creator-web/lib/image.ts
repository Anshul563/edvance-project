const ACCEPTED_IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp"]
const MAX_IMAGE_BYTES = 8 * 1024 * 1024
const MAX_DATA_URL_LENGTH = 400_000

export interface ImageCropOptions {
  width: number
  aspect: number
  quality?: number
}

/**
 * Reads an image file, centre-crops it to the requested aspect ratio and
 * returns a compact JPEG data URL. Keeping the image inline avoids adding
 * a second media pipeline for channel artwork.
 */
export async function imageFileToDataUrl(
  file: File,
  options: ImageCropOptions
): Promise<string> {
  if (!ACCEPTED_IMAGE_TYPES.includes(file.type)) {
    throw new Error("Choose a JPEG, PNG, or WebP image.")
  }

  if (file.size > MAX_IMAGE_BYTES) {
    throw new Error("Images must be smaller than 8 MB.")
  }

  const bitmap = await createImageBitmap(file)

  try {
    const targetRatio = options.aspect
    const sourceRatio = bitmap.width / bitmap.height

    let sourceWidth = bitmap.width
    let sourceHeight = bitmap.height

    if (sourceRatio > targetRatio) {
      sourceWidth = bitmap.height * targetRatio
    } else {
      sourceHeight = bitmap.width / targetRatio
    }

    const sourceX = (bitmap.width - sourceWidth) / 2
    const sourceY = (bitmap.height - sourceHeight) / 2
    const targetHeight = Math.max(1, Math.round(options.width / targetRatio))

    const canvas = document.createElement("canvas")
    canvas.width = options.width
    canvas.height = targetHeight

    const context = canvas.getContext("2d")
    if (!context) {
      throw new Error("This browser cannot process images.")
    }

    context.fillStyle = "#FFFFFF"
    context.fillRect(0, 0, canvas.width, canvas.height)
    context.drawImage(
      bitmap,
      sourceX,
      sourceY,
      sourceWidth,
      sourceHeight,
      0,
      0,
      canvas.width,
      canvas.height
    )

    let quality = options.quality ?? 0.82
    let dataUrl = canvas.toDataURL("image/jpeg", quality)

    while (dataUrl.length > MAX_DATA_URL_LENGTH && quality > 0.4) {
      quality -= 0.1
      dataUrl = canvas.toDataURL("image/jpeg", quality)
    }

    return dataUrl
  } finally {
    bitmap.close()
  }
}
