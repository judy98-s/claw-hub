/**
 * 제출 전에 브라우저에서 사진을 줄인다.
 *
 * 요즘 폰 사진은 한 장에 4~8MB다. 지하 매장 LTE에서 원본 세 장을 올리면
 * 제출이 타임아웃으로 죽고, 손님은 왜인지 모른 채 다시 시도한다.
 * 긴 변 1600px · JPEG 0.8 이면 장당 300KB 안팎이고, 증상 확인에는 충분하다.
 */

const MAX_EDGE = 1600;
const QUALITY = 0.8;

export async function shrinkImage(file: File): Promise<File> {
  // 이미지가 아니면 건드리지 않는다. 서버가 매직바이트로 다시 검사한다.
  if (!file.type.startsWith("image/")) return file;

  try {
    const bitmap = await createImageBitmap(file);
    const scale = Math.min(1, MAX_EDGE / Math.max(bitmap.width, bitmap.height));

    // 이미 충분히 작으면 재인코딩하지 않는다. 다시 굽는 건 화질만 깎는다.
    if (scale === 1 && file.size <= 1_000_000) {
      bitmap.close();
      return file;
    }

    const w = Math.round(bitmap.width * scale);
    const h = Math.round(bitmap.height * scale);

    const canvas = document.createElement("canvas");
    canvas.width = w;
    canvas.height = h;

    const ctx = canvas.getContext("2d");
    if (!ctx) {
      bitmap.close();
      return file;
    }
    ctx.drawImage(bitmap, 0, 0, w, h);
    bitmap.close();

    const blob = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, "image/jpeg", QUALITY),
    );
    if (!blob) return file;

    // 줄인 게 더 크면 원본을 쓴다. 작은 PNG 를 JPEG 로 다시 구우면 커질 수 있다.
    if (blob.size >= file.size) return file;

    return new File([blob], file.name.replace(/\.[^.]+$/, "") + ".jpg", {
      type: "image/jpeg",
      lastModified: Date.now(),
    });
  } catch {
    // HEIC 처럼 브라우저가 디코드 못 하는 포맷이 있다. 원본을 그대로 보낸다.
    return file;
  }
}
