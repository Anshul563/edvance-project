import { useState } from 'react'
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { useNavigate } from "react-router-dom"
import { login, logout, api } from "@/api-client"

export function LoginForm() {
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const handleLogin = async (e: Event) => {
    e.preventDefault()
    setLoading(true)
    try {
      const result = await api.login(email, password)
      if (result.success && result.data?.token) {
        // Token is stored in the api client
        navigate("/my-courses")
      } else {
        alert(result.error?.message || "Login failed")
      }
    } catch (err) {
      alert(err instanceof Error ? err.message : "Login failed")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-md space-y-4">
      <h2 className="text-lg font-medium">Sign in</h2>
      <form onSubmit={handleLogin} className="space-y-4">
        <Input
          type="email"
          placeholder="Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
        <Input
          type="password"
          placeholder="Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
        <Button type="submit" disabled={loading}>
          {loading ? "Signing in..." : "Sign in"}
        </Button>
      </form>
    </div>
  )
}

export function LogoutButton() {
  const handleLogout = async () => {
    await api.logout()
    // Navigate to login page or home
    // navigate("/login") // Uncomment if you have login route
  }

  return (
    <Button onClick={handleLogout}>
      Logout
    </Button>
  }
}
