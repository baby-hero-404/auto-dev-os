import type { Metadata } from "next";
import "./globals.css";
import { SWRProvider } from "@/lib/swr-config";

export const metadata: Metadata = {
  title: "Auto Code OS",
  description: "AI-native SDLC dashboard",
  icons: {
    icon: "/favicon.png",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full">
        <SWRProvider>{children}</SWRProvider>
      </body>
    </html>
  );
}

