import type { Metadata } from "next";
import "./globals.css";

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
      <body className="min-h-full">{children}</body>
    </html>
  );
}
