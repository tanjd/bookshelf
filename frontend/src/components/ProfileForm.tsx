"use client"

import { useEffect, useState, type FormEvent } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { api, validatePassword } from "@/lib/api"
import type { User } from "@/lib/types"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card"

export function ProfileForm() {
  const router = useRouter()
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  // Profile form
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [phone, setPhone] = useState("")
  const [saving, setSaving] = useState(false)

  // Change password
  const [currentPassword, setCurrentPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmNewPassword, setConfirmNewPassword] = useState("")
  const [pwError, setPwError] = useState("")
  const [changingPw, setChangingPw] = useState(false)

  // OTP
  const [otpSent, setOtpSent] = useState(false)
  const [otpCode, setOtpCode] = useState("")
  const [sendingOtp, setSendingOtp] = useState(false)
  const [verifyingOtp, setVerifyingOtp] = useState(false)

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token")
    if (!token) {
      router.push("/login")
      return
    }
    api.me()
      .then((u) => {
        setUser(u)
        setName(u.name)
        setEmail(u.email)
        setPhone(u.phone?.startsWith("+65") ? u.phone.slice(3).trim() : (u.phone ?? ""))
      })
      .catch(() => router.push("/login"))
      .finally(() => setLoading(false))
  }, [router])

  async function handleSaveProfile(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    try {
      const localPhone = phone.trim()
      const fullPhone = localPhone
        ? localPhone.startsWith("+") ? localPhone : `+65 ${localPhone}`
        : undefined
      const updated = await api.updateMe({
        name: name.trim() || undefined,
        email: email.trim() || undefined,
        phone: fullPhone,
      })
      setUser(updated)
      localStorage.setItem("bookshelf_user", JSON.stringify(updated))
      toast.success("Profile updated")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to update profile")
    } finally {
      setSaving(false)
    }
  }

  async function handleChangePassword(e: FormEvent) {
    e.preventDefault()
    setPwError("")
    const validationError = validatePassword(newPassword)
    if (validationError) { setPwError(validationError); return }
    if (newPassword !== confirmNewPassword) { setPwError("New passwords do not match"); return }
    setChangingPw(true)
    try {
      await api.changePassword({
        current_password: currentPassword,
        new_password: newPassword,
        confirm_password: confirmNewPassword,
      })
      setCurrentPassword("")
      setNewPassword("")
      setConfirmNewPassword("")
      toast.success("Password changed successfully")
    } catch (err) {
      setPwError(err instanceof Error ? err.message : "Failed to change password")
    } finally {
      setChangingPw(false)
    }
  }

  async function handleSendOTP() {
    setSendingOtp(true)
    try {
      await api.sendOTP()
      setOtpSent(true)
      toast.success("Verification code sent to your email")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to send code")
    } finally {
      setSendingOtp(false)
    }
  }

  async function handleVerifyOTP(e: FormEvent) {
    e.preventDefault()
    setVerifyingOtp(true)
    try {
      const updated = await api.verifyOTP(otpCode.trim())
      setUser(updated)
      localStorage.setItem("bookshelf_user", JSON.stringify(updated))
      setOtpSent(false)
      setOtpCode("")
      toast.success("Email verified!")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Invalid or expired code")
    } finally {
      setVerifyingOtp(false)
    }
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-6 max-w-md mx-auto">
        <div className="flex flex-col items-center gap-4 py-8">
          <div className="size-20 rounded-full bg-muted animate-pulse" />
          <div className="flex flex-col items-center gap-2">
            <div className="h-7 w-36 rounded bg-muted animate-pulse" />
            <div className="h-4 w-48 rounded bg-muted animate-pulse" />
            <div className="h-5 w-20 rounded-full bg-muted animate-pulse" />
          </div>
        </div>
        <div className="h-48 rounded-xl bg-muted animate-pulse" />
        <div className="h-24 rounded-xl bg-muted animate-pulse" />
      </div>
    )
  }

  if (!user) return null

  return (
    <div className="flex flex-col gap-6 max-w-md mx-auto">
      {/* Hero */}
      <div className="flex flex-col items-center gap-4 py-8 text-center">
        <div className="size-20 rounded-full bg-primary/10 flex items-center justify-center text-3xl font-bold text-primary select-none">
          {user.name.charAt(0).toUpperCase()}
        </div>
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-bold">{user.name}</h1>
          <p className="text-sm text-muted-foreground">{user.email}</p>
        </div>
        {user.verified
          ? <Badge variant="success">Verified</Badge>
          : <Badge variant="secondary">Unverified</Badge>}
      </div>

      {/* Profile form */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Personal information</CardTitle>
          <CardDescription>Update your name, email, and phone number</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSaveProfile} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <label htmlFor="profile-name" className="text-sm font-medium">Name</label>
              <Input id="profile-name" type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="Your name" />
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="profile-email" className="text-sm font-medium">Email</label>
              <Input id="profile-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@example.com" />
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="profile-phone" className="text-sm font-medium">Phone</label>
              <div className="flex rounded-md border border-input overflow-hidden focus-within:ring-1 focus-within:ring-ring">
                <span className="flex items-center px-3 bg-muted text-muted-foreground text-sm border-r border-input select-none">+65</span>
                <input
                  id="profile-phone"
                  type="tel"
                  className="flex-1 px-3 py-2 text-sm outline-none bg-background"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  placeholder="9123 4567"
                />
              </div>
            </div>
            <Button type="submit" disabled={saving}>{saving ? "Saving…" : "Save changes"}</Button>
          </form>
        </CardContent>
      </Card>

      {/* Change password */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Change password</CardTitle>
          <CardDescription>Update your password. You must enter your current password to confirm.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleChangePassword} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <label htmlFor="current-password" className="text-sm font-medium">Current password</label>
              <Input id="current-password" type="password" autoComplete="current-password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} placeholder="Your current password" />
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="new-password" className="text-sm font-medium">New password</label>
              <Input id="new-password" type="password" autoComplete="new-password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder="At least 8 characters" />
              <p className="text-xs text-muted-foreground">At least 8 characters with uppercase, lowercase, and a number.</p>
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="confirm-new-password" className="text-sm font-medium">Confirm new password</label>
              <Input id="confirm-new-password" type="password" autoComplete="new-password" value={confirmNewPassword} onChange={(e) => setConfirmNewPassword(e.target.value)} placeholder="Re-enter new password" />
            </div>
            {pwError && <p className="text-sm text-destructive">{pwError}</p>}
            <Button type="submit" disabled={changingPw || !currentPassword || !newPassword || !confirmNewPassword}>
              {changingPw ? "Changing…" : "Change password"}
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* Verification */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Email verification</CardTitle>
          <CardDescription>
            {user.verified ? "Your email address has been verified." : "Verify your email to unlock borrowing features."}
          </CardDescription>
        </CardHeader>
        {!user.verified && (
          <CardContent className="flex flex-col gap-4">
            {!otpSent ? (
              <Button variant="outline" onClick={handleSendOTP} disabled={sendingOtp}>
                {sendingOtp ? "Sending…" : "Send verification code"}
              </Button>
            ) : (
              <form onSubmit={handleVerifyOTP} className="flex flex-col gap-3">
                <p className="text-sm text-muted-foreground">
                  A 6-digit code was sent to <strong>{user.email}</strong>.
                </p>
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="otp-code" className="text-sm font-medium">Verification code</label>
                  <Input id="otp-code" type="text" inputMode="numeric" maxLength={6} value={otpCode} onChange={(e) => setOtpCode(e.target.value)} placeholder="123456" />
                </div>
                <div className="flex gap-2">
                  <Button type="submit" disabled={verifyingOtp || otpCode.length !== 6}>
                    {verifyingOtp ? "Verifying…" : "Verify"}
                  </Button>
                  <Button type="button" variant="ghost" onClick={handleSendOTP} disabled={sendingOtp}>
                    Resend code
                  </Button>
                </div>
              </form>
            )}
          </CardContent>
        )}
      </Card>
    </div>
  )
}
