param( $param1, $param2 )
# ��鲢�Թ���Ա�������PS�����ϲ���
$currentWi = [Security.Principal.WindowsIdentity]::GetCurrent()
$currentWp = [Security.Principal.WindowsPrincipal]$currentWi
if( -not $currentWp.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator))
{
    $boundPara = ($MyInvocation.BoundParameters.Keys | foreach{'-{0} {1}' -f  $_ ,$MyInvocation.BoundParameters[$_]} ) -join ' '
    $currentFile = $MyInvocation.MyCommand.Definition
    $fullPara = $boundPara + ' ' + $args -join ' '
    Start-Process "$psHome\powershell.exe"   -ArgumentList "$currentFile $fullPara"   -verb runas
    return
}
# ��ȡ��ǰ�ļ�·��
$currentpath = Split-Path -Parent $MyInvocation.MyCommand.Definition
$ico_path = $currentpath + "\kaf-cli.exe"
$exe_path = $currentpath + "\kaf-cli.exe" + ' "%1"'

# �����Ҽ��˵�
New-Item -Force -Path Registry::HKEY_CLASSES_ROOT\txtfile\shell\ʹ��kaf-cliת��
New-ItemProperty -Force -Path Registry::HKEY_CLASSES_ROOT\txtfile\shell\ʹ��kaf-cliת�� -Name Icon -PropertyType String -Value $ico_path

New-Item -Force -Path Registry::HKEY_CLASSES_ROOT\txtfile\shell\ʹ��kaf-cliת��\command
New-ItemProperty -Force -Path Registry::HKEY_CLASSES_ROOT\txtfile\shell\ʹ��kaf-cliת��\command -Name "(default)" -PropertyType String -Value $exe_path

echo "ע���Ҽ��˵��ɹ�!"
pause
 
