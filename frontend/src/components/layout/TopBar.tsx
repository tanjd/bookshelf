"use client";

import { Eye, EyeOff, Menu } from "lucide-react";
import { useYear } from "@/context/YearContext";
import { usePrivacy } from "@/context/PrivacyContext";
import { useMobileNav } from "@/context/MobileNavContext";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { UploadDialog } from "./UploadDialog";

export function TopBar() {
  const { years, selectedYear, setSelectedYear } = useYear();
  const { privacyMode, togglePrivacyMode } = usePrivacy();
  const { openNav } = useMobileNav();

  return (
    <header className="flex h-14 items-center justify-between border-b px-4">
      <button
        className="md:hidden rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-accent-foreground"
        onClick={openNav}
        aria-label="Open navigation"
      >
        <Menu className="h-5 w-5" />
      </button>
      <div className="hidden md:block" />
      <div className="flex items-center gap-3">
        {years.length > 0 && (
          <Select
            value={selectedYear?.toString() ?? ""}
            onValueChange={(v) => setSelectedYear(Number(v))}
          >
            <SelectTrigger className="h-8 w-28 text-sm">
              <SelectValue placeholder="Year" />
            </SelectTrigger>
            <SelectContent>
              {[...years].sort((a, b) => b - a).map((y) => (
                <SelectItem key={y} value={y.toString()}>
                  {y}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          onClick={togglePrivacyMode}
          title={privacyMode ? "Show values" : "Hide values"}
        >
          {privacyMode ? (
            <EyeOff className="h-4 w-4 text-muted-foreground" />
          ) : (
            <Eye className="h-4 w-4 text-muted-foreground" />
          )}
        </Button>
        <UploadDialog />
      </div>
    </header>
  );
}
