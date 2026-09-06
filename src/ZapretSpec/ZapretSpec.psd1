@{
    RootModule        = 'ZapretSpec.psm1'
    ModuleVersion     = '0.1.0'
    GUID              = '8c3e2a17-4b9d-4f1e-9a6c-2d7e5b8f1c04'
    Author            = 'Zapret Manager'
    Description       = 'Strategy JSON to winws / winws2 argv. Load this module by path. Do not install in PSModulePath.'
    PowerShellVersion = '3.0'
    FunctionsToExport = @(
        'Get-ZapretSpecVersion'
        'Get-ZapretStrategySpec'
        'ConvertTo-ZapretStrategyArgList'
    )
    CmdletsToExport   = @()
    VariablesToExport = @()
    AliasesToExport   = @()
}
