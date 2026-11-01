package dev.kyouji.tunnelgo

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.text.InputType
import android.view.ViewGroup
import android.widget.Button
import android.widget.CheckBox
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView

class MainActivity : Activity() {
    private val fields = mutableMapOf<String, EditText>()
    private lateinit var tls: CheckBox
    private lateinit var skipTLS: CheckBox
    private lateinit var skipSSH: CheckBox
    private lateinit var waitResponse: CheckBox

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val layout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(24, 28, 24, 24)
        }
        layout.addView(TextView(this).apply { text = "TUNNEL-GO"; textSize = 26f })
        layout.addView(TextView(this).apply { text = "SSH transport • HEV TUN"; textSize = 13f })

        addInput(layout, "address", "SSH address (host:port)")
        addInput(layout, "username", "SSH username")
        addInput(layout, "password", "SSH password", true)
        addInput(layout, "proxy", "HTTP proxy address (optional)")
        addInput(layout, "sni", "TLS SNI (optional)")
        addInput(layout, "payload", "Custom payload (optional)", multiline = true)

        tls = CheckBox(this).apply { text = "Use TLS" }
        skipTLS = CheckBox(this).apply { text = "Skip TLS certificate verification" }
        skipSSH = CheckBox(this).apply { text = "Skip SSH host-key verification (testing only)" }
        waitResponse = CheckBox(this).apply { text = "Wait for HTTP proxy response" }
        listOf(tls, skipTLS, skipSSH, waitResponse).forEach(layout::addView)

        layout.addView(Button(this).apply {
            text = "CONNECT"
            setOnClickListener { requestVpnPermission() }
        })
        layout.addView(Button(this).apply {
            text = "DISCONNECT"
            setOnClickListener {
                startService(Intent(this@MainActivity, TunnelVpnService::class.java).setAction(TunnelVpnService.ACTION_STOP))
            }
        })
        setContentView(ScrollView(this).apply { addView(layout) })
    }

    private fun addInput(parent: LinearLayout, key: String, hint: String, secret: Boolean = false, multiline: Boolean = false) {
        val input = EditText(this).apply {
            this.hint = hint
            inputType = when {
                multiline -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_MULTI_LINE
                secret -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
                else -> InputType.TYPE_CLASS_TEXT
            }
            if (multiline) minLines = 3
            layoutParams = LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT)
        }
        fields[key] = input
        parent.addView(input)
    }

    private fun requestVpnPermission() {
        val permission = VpnService.prepare(this)
        if (permission == null) startTunnel() else startActivityForResult(permission, REQUEST_VPN)
    }

    @Deprecated("Compatibility with the minimum Android API")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == REQUEST_VPN && resultCode == RESULT_OK) startTunnel()
    }

    private fun startTunnel() {
        val intent = Intent(this, TunnelVpnService::class.java).apply {
            action = TunnelVpnService.ACTION_START
            putExtra(TunnelVpnService.EXTRA_ADDRESS, value("address"))
            putExtra(TunnelVpnService.EXTRA_USERNAME, value("username"))
            putExtra(TunnelVpnService.EXTRA_PASSWORD, value("password"))
            putExtra(TunnelVpnService.EXTRA_PROXY, value("proxy"))
            putExtra(TunnelVpnService.EXTRA_SNI, value("sni"))
            putExtra(TunnelVpnService.EXTRA_PAYLOAD, value("payload"))
            putExtra(TunnelVpnService.EXTRA_TLS, tls.isChecked)
            putExtra(TunnelVpnService.EXTRA_INSECURE_TLS, skipTLS.isChecked)
            putExtra(TunnelVpnService.EXTRA_INSECURE_SSH, skipSSH.isChecked)
            putExtra(TunnelVpnService.EXTRA_WAIT_RESPONSE, waitResponse.isChecked)
        }
        if (Build.VERSION.SDK_INT >= 26) startForegroundService(intent) else startService(intent)
    }

    private fun value(key: String) = fields[key]?.text?.toString()?.trim().orEmpty()

    companion object {
        private const val REQUEST_VPN = 7001
    }
}
