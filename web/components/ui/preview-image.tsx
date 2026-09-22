"use client";

import { useEffect, useState } from "react";

/**
 * 파일 하나의 미리보기.
 *
 * URL.createObjectURL 은 revoke 하기 전까지 그 파일 전체를 메모리에 붙들고
 * 있는다. 렌더 안에서 부르면 리렌더마다 새 URL이 생기고 옛것은 아무도
 * 해제하지 않는다. 사진 세 장을 올린 채 폼을 만지작거리면 그만큼 쌓인다.
 *
 * 그래서 URL 을 effect 안에서 한 번만 만들고, 파일이 바뀌거나 화면에서
 * 사라질 때 반드시 해제한다.
 */
export function PreviewImage({
  file,
  alt,
  className = "",
}: {
  file: File;
  alt: string;
  className?: string;
}) {
  const [url, setUrl] = useState("");

  useEffect(() => {
    const next = URL.createObjectURL(file);
    setUrl(next);
    return () => URL.revokeObjectURL(next);
  }, [file]);

  // 첫 렌더에는 아직 URL이 없다. 자리를 비워두면 목록이 한 번 튄다.
  if (!url)
    return <div className={`bg-[var(--surface-sunken)] ${className}`} />;

  // eslint-disable-next-line @next/next/no-img-element
  return <img src={url} alt={alt} className={className} />;
}
