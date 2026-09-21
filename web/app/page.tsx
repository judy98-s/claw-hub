import { redirect } from "next/navigation";

/** 루트는 사장님 대시보드로 보낸다. 손님은 QR로 /r/{code} 에 바로 들어온다. */
export default function Home() {
  redirect("/admin");
}
