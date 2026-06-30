export const metadata = {
  title: "BrowserAgent",
  description: "BrowserAgent web app",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
