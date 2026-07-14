import type { Metadata } from "next";
import "./globals.css";
import { SWRProvider } from "@/lib/swr-config";
import { ThemeProvider } from "next-themes";

export const metadata: Metadata = {
  title: "Auto Code OS",
  description: "AI-native SDLC dashboard",
  icons: {
    icon: "/favicon.png",
  },
};

import { Toaster } from "sonner";

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full antialiased" suppressHydrationWarning>
      <body className="min-h-full">
        <ThemeProvider attribute="class" defaultTheme="light" enableSystem disableTransitionOnChange>
          <SWRProvider>{children}</SWRProvider>
          <Toaster position="top-right" theme="system" />
        </ThemeProvider>
      </body>
    </html>
  );
}
